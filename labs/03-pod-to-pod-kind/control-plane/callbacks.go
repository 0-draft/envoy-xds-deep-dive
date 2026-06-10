package main

import (
	"context"
	"log"
	"path"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

type logger struct{}

func (logger) Debugf(string, ...interface{})    {}
func (logger) Infof(string, ...interface{})     {}
func (logger) Warnf(f string, a ...interface{}) { log.Printf("WARN  "+f, a...) }
func (logger) Errorf(f string, a ...interface{}) {
	log.Printf("ERROR "+f, a...)
}

// callbacks logs which node ACKs/NACKs each push, so you can watch both sidecars
// converge on the same control plane stream.
type callbacks struct{}

func (callbacks) OnStreamOpen(_ context.Context, id int64, _ string) error {
	log.Printf("stream %d open", id)
	return nil
}
func (callbacks) OnStreamClosed(id int64, node *corev3.Node) {
	log.Printf("stream %d closed (node %q)", id, node.GetId())
}
func (callbacks) OnStreamRequest(id int64, req *discovery.DiscoveryRequest) error {
	short := path.Base(req.GetTypeUrl())
	node := req.GetNode().GetId()
	switch {
	case req.GetErrorDetail() != nil:
		log.Printf("stream %d node=%s NACK %-22s: %s", id, node, short, req.GetErrorDetail().GetMessage())
	case req.GetResponseNonce() == "":
		log.Printf("stream %d node=%s  REQ %-22s (initial)", id, node, short)
	default:
		log.Printf("stream %d node=%s  ACK %-22s version=%q", id, node, short, req.GetVersionInfo())
	}
	return nil
}
func (callbacks) OnStreamResponse(context.Context, int64, *discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {
}
func (callbacks) OnFetchRequest(context.Context, *discovery.DiscoveryRequest) error { return nil }
func (callbacks) OnFetchResponse(*discovery.DiscoveryRequest, *discovery.DiscoveryResponse) {}
func (callbacks) OnDeltaStreamOpen(context.Context, int64, string) error             { return nil }
func (callbacks) OnDeltaStreamClosed(int64, *corev3.Node)                            {}
func (callbacks) OnStreamDeltaRequest(int64, *discovery.DeltaDiscoveryRequest) error { return nil }
func (callbacks) OnStreamDeltaResponse(int64, *discovery.DeltaDiscoveryRequest, *discovery.DeltaDiscoveryResponse) {
}
