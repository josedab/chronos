// Package tracing provides Raft operation span helpers.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Raft span attribute keys.
var (
	AttrRaftState  = attribute.Key("chronos.raft.state")
	AttrRaftTerm   = attribute.Key("chronos.raft.term")
	AttrRaftPeerID = attribute.Key("chronos.raft.peer_id")
)

// StartRaftLeaderElectionSpan starts a span for a Raft leader election event.
func StartRaftLeaderElectionSpan(ctx context.Context, nodeID string, isLeader bool) (context.Context, trace.Span) {
	state := "follower"
	if isLeader {
		state = "leader"
	}
	return tracer.Start(ctx, "raft.leader_election",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			AttrNodeID.String(nodeID),
			AttrRaftState.String(state),
			AttrIsLeader.Bool(isLeader),
		),
	)
}

// StartRaftApplySpan starts a span for a Raft log apply operation.
func StartRaftApplySpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "raft.apply",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("chronos.raft.operation", operation),
		),
	)
}

// StartRaftSnapshotSpan starts a span for a Raft snapshot operation.
func StartRaftSnapshotSpan(ctx context.Context, nodeID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "raft.snapshot",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			AttrNodeID.String(nodeID),
		),
	)
}
