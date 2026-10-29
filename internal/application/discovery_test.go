// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package application_test

import (
	"aws-architecture-discovery/internal/application"
	"aws-architecture-discovery/internal/domain"
	"context"
	"errors"
	"testing"
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
