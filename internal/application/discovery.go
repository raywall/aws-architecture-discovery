// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package application orchestrates discovery through injected ports.
package application

import (
	"aws-architecture-discovery/internal/domain"
	"context"
	"fmt"
)

// Provider supplies one complete topology snapshot or an error.
type Provider interface {
	// Collect defines the Collect operation contract.
	Collect(context.Context) (domain.Graph, error)
}

// Renderer encodes a resolved graph.
type Renderer interface {
	// Render defines the Render operation contract.
	Render(domain.Graph) ([]byte, error)
	// Extension defines the Extension operation contract.
	Extension() string
	// ContentType defines the ContentType operation contract.
	ContentType() string
}

// Sink persists an artifact and returns its location.
type Sink interface {
	// Write defines the Write operation contract.
	Write(context.Context, string, []byte, string) (string, error)
}

// Discovery coordinates collection, identity resolution, rendering and persistence.
type Discovery struct {
	// Providers contains the enabled topology collectors.
	Providers []Provider
	// Renderer encodes the resolved topology.
	Renderer Renderer
	// Sink persists the completed artifact.
	Sink Sink
}

// Execute fails closed when a collector fails, avoiding misleading partial artifacts.
func (d Discovery) Execute(ctx context.Context) (string, error) {
	parts := []domain.Graph{}
	for i, p := range d.Providers {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		g, err := p.Collect(ctx)
		if err != nil {
			return "", fmt.Errorf("provider %d: %w", i, err)
		}
		parts = append(parts, g)
	}
	data, err := d.Renderer.Render(domain.Resolve(parts))
	if err != nil {
		return "", err
	}
	return d.Sink.Write(ctx, "architecture."+d.Renderer.Extension(), data, d.Renderer.ContentType())
}
