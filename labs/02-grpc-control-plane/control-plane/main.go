// A minimal xDS control plane built on go-control-plane.
//
// It serves LDS + RDS + CDS + EDS to Envoy over a single ADS gRPC stream, and
// exposes a tiny HTTP admin so you can mutate the desired state and watch Envoy
// ACK (or NACK) the push in real time.
//
//	gRPC ADS   :18000   <- Envoy connects here
//	HTTP admin :19000   <- you poke here to change state
//
//	  POST /scale?n=1|2   set the EDS endpoint count (push EDS)
//	  POST /break         push an invalid Listener (port 70000) -> Envoy NACKs LDS
//	  POST /heal          push a valid Listener again -> Envoy ACKs
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

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

// nodeID must match `node.id` in the Envoy bootstrap. The snapshot cache is keyed by it.
const nodeID = "lab02-node"

const goodPort = 10000
const badPort = 70000 // > 65535 -> Envoy rejects (NACK) the Listener

// All upstreams the control plane knows about (their fixed docker IPs).
var allUpstreams = []hostPort{
	{host: "10.78.0.11", port: 5678},
	{host: "10.78.0.12", port: 5678},
}

func main() {
	ctx := context.Background()
	cache := cachev3.NewSnapshotCache(true, cachev3.IDHash{}, logger{})

	state := &desiredState{n: 2, port: goodPort}

	// Publish the initial snapshot (version 1).
	var version int64
	if err := publish(ctx, cache, &version, state); err != nil {
		log.Fatalf("initial snapshot: %v", err)
	}

	// gRPC ADS server.
	srv := serverv3.NewServer(ctx, cache, callbacks{})
	grpcServer := grpc.NewServer()
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	// Registering the per-type services too lets non-ADS clients work, but ADS
	// (above) is what this lab uses.
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, srv)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, srv)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, srv)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", ":18000")
	if err != nil {
		log.Fatalf("listen 18000: %v", err)
	}
	go func() {
		log.Printf("xDS ADS server listening on :18000 (node %q)", nodeID)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	// HTTP admin to drive the desired state.
	http.HandleFunc("/scale", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n < 1 || n > len(allUpstreams) {
			http.Error(w, fmt.Sprintf("n must be 1..%d", len(allUpstreams)), http.StatusBadRequest)
			return
		}
		state.set(n, goodPort)
		mustPublish(ctx, cache, &version, state, w)
	})
	http.HandleFunc("/break", func(w http.ResponseWriter, r *http.Request) {
		state.set(state.count(), badPort)
		mustPublish(ctx, cache, &version, state, w)
	})
	http.HandleFunc("/heal", func(w http.ResponseWriter, r *http.Request) {
		state.set(state.count(), goodPort)
		mustPublish(ctx, cache, &version, state, w)
	})

	log.Printf("HTTP admin listening on :19000 (/scale?n=, /break, /heal)")
	log.Fatal(http.ListenAndServe(":19000", nil))
}

// desiredState is the small, mutable "intent" the control plane converts into a snapshot.
type desiredState struct {
	mu   sync.Mutex
	n    int
	port uint32
}

func (s *desiredState) set(n int, port uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n, s.port = n, port
}

func (s *desiredState) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.n
}

func (s *desiredState) snapshotInputs() ([]hostPort, uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return allUpstreams[:s.n], s.port
}

// publish builds a new snapshot from the desired state and sets it (a push).
func publish(ctx context.Context, cache cachev3.SnapshotCache, version *int64, state *desiredState) error {
	v := atomic.AddInt64(version, 1)
	addrs, port := state.snapshotInputs()
	clusters, endpoints, routes, listeners := resourcesFor(addrs, port)

	snap, err := cachev3.NewSnapshot(strconv.FormatInt(v, 10), map[resourcev3.Type][]types.Resource{
		resourcev3.ClusterType:  clusters,
		resourcev3.EndpointType: endpoints,
		resourcev3.RouteType:    routes,
		resourcev3.ListenerType: listeners,
	})
	if err != nil {
		return err
	}
	if err := snap.Consistent(); err != nil {
		return err
	}
	log.Printf("PUSH  version=%d  endpoints=%d  listenerPort=%d", v, len(addrs), port)
	return cache.SetSnapshot(ctx, nodeID, snap)
}

func mustPublish(ctx context.Context, cache cachev3.SnapshotCache, version *int64, state *desiredState, w http.ResponseWriter) {
	if err := publish(ctx, cache, version, state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	addrs, port := state.snapshotInputs()
	fmt.Fprintf(w, "pushed: endpoints=%d listenerPort=%d\n", len(addrs), port)
}
