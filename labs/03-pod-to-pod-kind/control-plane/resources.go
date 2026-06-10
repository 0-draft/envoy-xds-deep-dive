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
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

type hostPort struct {
	host string
	port uint32
}

// adsSource: "discover this resource over the ADS stream we're already on".
func adsSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ResourceApiVersion:    corev3.ApiVersion_V3,
		ConfigSourceSpecifier: &corev3.ConfigSource_Ads{Ads: &corev3.AggregatedConfigSource{}},
	}
}

// edsCluster: a CDS resource whose endpoints arrive via EDS (used on the caller
// side, where endpoints are real pod IPs that churn).
func edsCluster(name string) *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name:                 name,
		ConnectTimeout:       durationpb.New(time.Second),
		ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS},
		LbPolicy:             clusterv3.Cluster_ROUND_ROBIN,
		EdsClusterConfig: &clusterv3.Cluster_EdsClusterConfig{
			ServiceName: name,
			EdsConfig:   adsSource(),
		},
	}
}

// staticCluster: a CDS resource with inline endpoints (used on the callee side
// to reach the local app over loopback — it never changes).
func staticCluster(name string, ep hostPort) *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name:                 name,
		ConnectTimeout:       durationpb.New(time.Second),
		ClusterDiscoveryType: &clusterv3.Cluster_Type{Type: clusterv3.Cluster_STATIC},
		LbPolicy:             clusterv3.Cluster_ROUND_ROBIN,
		LoadAssignment:       clusterLoadAssignment(name, []hostPort{ep}),
	}
}

// clusterLoadAssignment: an EDS resource (ClusterLoadAssignment) — the endpoint list.
func clusterLoadAssignment(name string, addrs []hostPort) *endpointv3.ClusterLoadAssignment {
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
		ClusterName: name,
		Endpoints:   []*endpointv3.LocalityLbEndpoints{{LbEndpoints: lbs}},
	}
}

// routeConfig: an RDS resource — send everything to one cluster.
func routeConfig(name, cluster string) *routev3.RouteConfiguration {
	return &routev3.RouteConfiguration{
		Name: name,
		VirtualHosts: []*routev3.VirtualHost{
			{
				Name:    cluster,
				Domains: []string{"*"},
				Routes: []*routev3.Route{
					{
						Match: &routev3.RouteMatch{PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: "/"}},
						Action: &routev3.Route_Route{
							Route: &routev3.RouteAction{
								ClusterSpecifier: &routev3.RouteAction_Cluster{Cluster: cluster},
							},
						},
					},
				},
			},
		},
	}
}

// httpListener: an LDS resource — listen on a port, pull routes via RDS.
func httpListener(name string, port uint32, routeName string) *listenerv3.Listener {
	router, _ := anypb.New(&routerv3.Router{})
	manager := &hcmv3.HttpConnectionManager{
		StatPrefix: name,
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{RouteConfigName: routeName, ConfigSource: adsSource()},
		},
		HttpFilters: []*hcmv3.HttpFilter{
			{Name: wellknown.Router, ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: router}},
		},
	}
	managerAny, _ := anypb.New(manager)
	return &listenerv3.Listener{
		Name: name,
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address:       "0.0.0.0",
					PortSpecifier: &corev3.SocketAddress_PortValue{PortValue: port},
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{
			{Filters: []*listenerv3.Filter{
				{Name: wellknown.HTTPConnectionManager, ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: managerAny}},
			}},
		},
	}
}
