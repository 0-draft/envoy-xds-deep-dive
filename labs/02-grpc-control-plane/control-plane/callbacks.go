package main

import (
	"context"
	"log"
	"path"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

// logger satisfies go-control-plane's cache.Logger; quiet by default.
type logger struct{}

func (logger) Debugf(string, ...interface{}) {}
func (logger) Infof(string, ...interface{})  {}
func (logger) Warnf(f string, a ...interface{}) {
	log.Printf("WARN  "+f, a...)
}
func (logger) Errorf(f string, a ...interface{}) {
	log.Printf("ERROR "+f, a...)
}

// callbacks implements server.Callbacks. The interesting one is OnStreamRequest:
// every message Envoy sends upstream on the ADS stream is a DiscoveryRequest, and
// it doubles as the ACK/NACK for the previous push.
//
//   - VersionInfo == ""            -> initial request for a type (not an ack)
//   - VersionInfo set, ErrorDetail nil  -> ACK of that version
//   - ErrorDetail != nil           -> NACK; Envoy rejected the push and tells us why
type callbacks struct{}

func (callbacks) OnStreamOpen(_ context.Context, id int64, typ string) error {
	log.Printf("stream %d open (%s)", id, orAll(typ))
	return nil
}

func (callbacks) OnStreamClosed(id int64, _ *corev3.Node) {
	log.Printf("stream %d closed", id)
}

func (callbacks) OnStreamRequest(id int64, req *discovery.DiscoveryRequest) error {
	short := path.Base(req.GetTypeUrl()) // e.g. "Listener", "ClusterLoadAssignment"
	switch {
	case req.GetErrorDetail() != nil:
		log.Printf("stream %d  NACK %-22s version=%q: %s",
			id, short, req.GetVersionInfo(), req.GetErrorDetail().GetMessage())
	case req.GetResponseNonce() == "":
		log.Printf("stream %d   REQ %-22s (initial)", id, short)
	default:
		log.Printf("stream %d   ACK %-22s version=%q", id, short, req.GetVersionInfo())
	}
	return nil
}

func (callbacks) OnStreamResponse(_ context.Context, id int64, _ *discovery.DiscoveryRequest, resp *discovery.DiscoveryResponse) {
	log.Printf("stream %d  SEND %-22s version=%q (%d resources)",
		id, path.Base(resp.GetTypeUrl()), resp.GetVersionInfo(), len(resp.GetResources()))
}

// Remaining interface methods are unused in this lab (ADS/SotW only).
func (callbacks) OnFetchRequest(context.Context, *discovery.DiscoveryRequest) error { return nil }
func (callbacks) OnFetchResponse(*discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {}
func (callbacks) OnDeltaStreamOpen(context.Context, int64, string) error               { return nil }
func (callbacks) OnDeltaStreamClosed(int64, *corev3.Node)                              {}
func (callbacks) OnStreamDeltaRequest(int64, *discovery.DeltaDiscoveryRequest) error   { return nil }
func (callbacks) OnStreamDeltaResponse(int64, *discovery.DeltaDiscoveryRequest, *discovery.DeltaDiscoveryResponse) {
}

func orAll(typ string) string {
	if typ == "" {
		return "ADS"
	}
	return path.Base(typ)
}
