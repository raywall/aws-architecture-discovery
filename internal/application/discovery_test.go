// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raywall/aws-architecture-discovery/internal/application"
	"github.com/raywall/aws-architecture-discovery/internal/domain"
)

type provider struct{ err error }

func (p provider) Collect(context.Context) (domain.Graph, error) {
	return domain.Graph{Nodes: []domain.Node{{ID: "a"}}}, p.err
}

type renderer struct{ err error }

func (r renderer) Render(domain.Graph) ([]byte, error) { return []byte("artifact"), r.err }
func (renderer) Extension() string                     { return "txt" }
func (renderer) ContentType() string                   { return "text/plain" }

type sink struct {
	written bool
	err     error
}

func (s *sink) Write(_ context.Context, name string, data []byte, mime string) (string, error) {
	s.written = true
	return name, s.err
}
func TestPipeline(t *testing.T) {
	t.Parallel()
	failure := errors.New("failure")
	for _, tc := range []struct {
		name            string
		p, r, s         error
		cancel, written bool
	}{{name: "success", written: true}, {name: "collection failure", p: failure}, {name: "render failure", r: failure}, {name: "write failure", s: failure, written: true}, {name: "cancelled", cancel: true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &sink{err: tc.s}
			app := application.Discovery{Providers: []application.Provider{provider{tc.p}}, Renderer: renderer{tc.r}, Sink: s}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			location, err := app.Execute(ctx)
			wantErr := tc.p != nil || tc.r != nil || tc.s != nil || tc.cancel
			if (err != nil) != wantErr || s.written != tc.written {
				t.Fatalf("err=%v written=%v", err, s.written)
			}
			if !wantErr && location != "architecture.txt" {
				t.Fatal(location)
			}
		})
	}
}

type graphProvider struct{ graph domain.Graph }

func (p graphProvider) Collect(context.Context) (domain.Graph, error) { return p.graph, nil }

type capturingRenderer struct{ graph domain.Graph }

func (r *capturingRenderer) Render(g domain.Graph) ([]byte, error) {
	r.graph = g
	return []byte("artifact"), nil
}
func (*capturingRenderer) Extension() string   { return "txt" }
func (*capturingRenderer) ContentType() string { return "text/plain" }

func TestPipelineAppliesExclusionsBeforeRendering(t *testing.T) {
	t.Parallel()
	r := &capturingRenderer{}
	app := application.Discovery{IgnoreComponents: []string{"ssm-parameters"}, Providers: []application.Provider{graphProvider{domain.Graph{
		Nodes: []domain.Node{{ID: "function", Kind: "lambda"}, {ID: "parameter", Kind: "ssm-parameter"}},
		Edges: []domain.Edge{{From: "function", To: "parameter", Kind: "references"}},
	}}}, Renderer: r, Sink: &sink{}}
	if _, err := app.Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.graph.Nodes) != 1 || r.graph.Nodes[0].ID != "function" || len(r.graph.Edges) != 0 {
		t.Fatalf("renderer received excluded components: %+v", r.graph)
	}
}
