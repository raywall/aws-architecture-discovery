// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package telemetry instruments application ports using native Datadog tracing.
package telemetry

import (
	"context"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/raywall/aws-architecture-discovery/internal/application"
	"github.com/raywall/aws-architecture-discovery/internal/domain"
)

// Provider decorates collection with a native span and topology counts.
type Provider struct {
	// Name is the human-readable resource label.
	Name string
	// Inner is the decorated topology collector.
	Inner application.Provider
}

// Collect propagates tracing context and records failures without logging metadata values.
func (p Provider) Collect(ctx context.Context) (g domain.Graph, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "discovery.collect", tracer.ResourceName(p.Name))
	defer func() {
		span.SetTag("discovery.nodes", len(g.Nodes))
		span.SetTag("discovery.edges", len(g.Edges))
		span.Finish(tracer.WithError(err))
	}()
	return p.Inner.Collect(ctx)
}
