// A mesh-aware xDS control plane for the pod-to-pod lab.
//
// It serves DIFFERENT config to two sidecars over ADS, keyed by node id:
//
//	node "app-a-sidecar" (the caller):
//	    LDS  outbound listener  :10000
//	    RDS  -> cluster "app-b"
//	    CDS  cluster "app-b" (EDS)
//	    EDS  app-b POD IPs : <inbound port>   (discovered live from headless DNS)
//
//	node "app-b-sidecar" (the callee):
//	    LDS  inbound listener   :15006
//	    RDS  -> cluster "app-local"
//	    CDS  cluster "app-local" (STATIC 127.0.0.1:<app port>)
//
// The caller's EDS is refreshed by resolving the app-b headless Service every
// few seconds, so scaling the app-b Deployment pushes new endpoints live.
package main

import (
	"context"
	"log"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
)

const (
	appANode = "app-a-sidecar"
	appBNode = "app-b-sidecar"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	var (
		appBDNS     = env("APP_B_DNS", "app-b.default.svc.cluster.local")
		inboundPort = uint32(mustAtoi(env("INBOUND_PORT", "15006")))
		outboundPt  = uint32(mustAtoi(env("OUTBOUND_PORT", "10000")))
		appPort     = uint32(mustAtoi(env("APP_PORT", "5678")))
	)

	ctx := context.Background()
	cache := cachev3.NewSnapshotCache(true, cachev3.IDHash{}, logger{})
	var version int64

	// --- callee (app-b) snapshot: static, set once. ---
	setSnapshot(ctx, cache, &version, appBNode,
		[]types.Resource{staticCluster("app-local", hostPort{"127.0.0.1", appPort})},
		nil, // no EDS (cluster is STATIC)
		[]types.Resource{routeConfig("inbound", "app-local")},
		[]types.Resource{httpListener("inbound", inboundPort, "inbound")},
	)

	// --- caller (app-a) snapshot: rebuilt whenever app-b endpoints change. ---
	updateCaller := func(addrs []hostPort) {
		setSnapshot(ctx, cache, &version, appANode,
			[]types.Resource{edsCluster("app-b")},
			[]types.Resource{clusterLoadAssignment("app-b", addrs)},
			[]types.Resource{routeConfig("outbound-to-app-b", "app-b")},
			[]types.Resource{httpListener("outbound-app-b", outboundPt, "outbound-to-app-b")},
		)
	}
	updateCaller(nil) // start with zero endpoints; the resolver fills them in

	// Resolve the app-b headless Service on a loop; push EDS when the set changes.
	go resolveLoop(appBDNS, inboundPort, updateCaller)

	// gRPC ADS server.
	srv := serverv3.NewServer(ctx, cache, callbacks{})
	grpcServer := grpc.NewServer()
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, srv)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, srv)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, srv)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", ":18000")
	if err != nil {
		log.Fatalf("listen 18000: %v", err)
	}
	log.Printf("mesh xDS ADS server on :18000 (nodes %q, %q); app-b DNS=%s",
		appANode, appBNode, appBDNS)
	log.Fatal(grpcServer.Serve(lis))
}

// resolveLoop looks up the headless Service every 3s and calls update when the
// sorted endpoint set changes. This is a stand-in for watching the k8s API.
func resolveLoop(dns string, port uint32, update func([]hostPort)) {
	var last []string
	for {
		ips, err := net.LookupHost(dns)
		if err != nil {
			log.Printf("resolve %s: %v", dns, err)
			time.Sleep(3 * time.Second)
			continue
		}
		sort.Strings(ips)
		if !reflect.DeepEqual(ips, last) {
			last = ips
			addrs := make([]hostPort, 0, len(ips))
			for _, ip := range ips {
				addrs = append(addrs, hostPort{ip, port})
			}
			log.Printf("app-b endpoints changed -> %v", ips)
			update(addrs)
		}
		time.Sleep(3 * time.Second)
	}
}

func setSnapshot(ctx context.Context, cache cachev3.SnapshotCache, version *int64, node string,
	clusters, endpoints, routes, listeners []types.Resource) {
	v := atomic.AddInt64(version, 1)
	snap, err := cachev3.NewSnapshot(strconv.FormatInt(v, 10), map[resourcev3.Type][]types.Resource{
		resourcev3.ClusterType:  clusters,
		resourcev3.EndpointType: endpoints,
		resourcev3.RouteType:    routes,
		resourcev3.ListenerType: listeners,
	})
	if err != nil {
		log.Printf("snapshot for %s: %v", node, err)
		return
	}
	if err := snap.Consistent(); err != nil {
		log.Printf("inconsistent snapshot for %s: %v", node, err)
		return
	}
	if err := cache.SetSnapshot(ctx, node, snap); err != nil {
		log.Printf("set snapshot for %s: %v", node, err)
		return
	}
	log.Printf("PUSH node=%s version=%d (cds=%d eds=%d rds=%d lds=%d resources)",
		node, v, len(clusters), len(endpoints), len(routes), len(listeners))
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("bad int %q: %v", s, err)
	}
	return n
}
