package main

import (
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Names that tie the four resource types together. Change one consistently
// across CDS/EDS or LDS/RDS and you have a working config; mismatch them and
// Envoy NACKs. That linkage is the whole point of this lab.
const (
	clusterName   = "service_backend"
	routeName     = "local_route"
	listenerName  = "listener_http"
	virtualHost   = "backend"
	listenPort    = 10000
)

// adsSource returns a ConfigSource that means "discover this over the same ADS
// stream you're already on". Both the Listener->RDS and Cluster->EDS links use it.
func adsSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ResourceApiVersion:    corev3.ApiVersion_V3,
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{Ads: &corev3.AggregatedConfigSource{}},
	}
}

// makeCluster builds a CDS resource: a cluster whose endpoints come from EDS.
func makeCluster() *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       durationpb.New(time.Second),
		ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS},
		LbPolicy:             clusterv3.Cluster_ROUND_ROBIN,
		EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
			ServiceName: clusterName,
			EdsConfig:   adsSource(),
		},
	}
}

// makeEndpoints builds an EDS resource: the concrete endpoint IPs for the cluster.
// This is the list that changes when pods scale; the control plane re-pushes it.
func makeEndpoints(addrs []hostPort) *endpointv3.ClusterLoadAssignment {
	lbs := make([]*endpointv3.LbEndpoint, 0, len(addrs))
	for _, a := range addrs {
		lbs = append(lbs, &endpointv3.LbEndpoint{
			HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
				Endpoint: &endpointv3.Endpoint{
					Address: &corev3.Address{
						Address: &corev3.Address_SocketAddress{
							SocketAddress: &corev3.SocketAddress{
								Address:       a.host,
								PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: a.port},
							},
						},
					},
				},
			},
		})
	}
	return &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{LbEndpoints: lbs},
		},
	}
}

// makeRoute builds an RDS resource: route everything to the cluster.
func makeRoute() *routev3.RouteConfiguration {
	return &routev3.RouteConfiguration{
		Name: routeName,
		VirtualHosts: []*routev3.VirtualHost{
			{
				Name:    virtualHost,
				Domains: []string{"*"},
				Routes: []*routev3.Route{
					{
						Match: &routev3.RouteMatch{PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"}},
						Action: &routev3.Route_Route{
							Route: &routev3.RouteAction{
								ClusterSpecifier: &routev3.RouteAction_Cluster{Cluster: clusterName},
							},
						},
					},
				},
			},
		},
	}
}

// makeListener builds an LDS resource: a listener whose HCM pulls its routes via RDS.
// `port` is a parameter so the lab can push an out-of-range port (e.g. 70000) and
// watch Envoy NACK the LDS update while CDS/EDS keep ACKing.
func makeListener(port uint32) *listenerv3.Listener {
	router, _ := anypb.New(&routerv3.Router{})
	manager := &hcmv3.HttpConnectionManager{
		StatPrefix: "ingress_http",
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{
				RouteConfigName: routeName,
				ConfigSource:    adsSource(),
			},
		},
		HttpFilters: []*hcmv3.HttpFilter{
			{
				Name:       wellknown.Router,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: router},
			},
		},
	}
	managerAny, _ := anypb.New(manager)

	return &listenerv3.Listener{
		Name: listenerName,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address:       "0.0.0.0",
					PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: port},
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{
				Filters: []*listenerv3.Filter{
					{
						Name:       wellknown.HTTPConnectionManager,
						ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: managerAny},
					},
				},
			},
		},
	}
}

type hostPort struct {
	host string
	port uint32
}

// resourcesFor returns the full set of resources for a snapshot, given the
// current endpoint list and listener port. Bumping `addrs` and re-snapshotting
// is an EDS push; passing a bad `port` produces an LDS NACK.
func resourcesFor(addrs []hostPort, port uint32) (clusters, endpoints, routes, listeners []types.Resource) {
	return []types.Resource{makeCluster()},
		[]types.Resource{makeEndpoints(addrs)},
		[]types.Resource{makeRoute()},
		[]types.Resource{makeListener(port)}
}
