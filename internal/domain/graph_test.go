// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package domain_test

import (
	"aws-architecture-discovery/internal/domain"
	"reflect"
	"testing"
)

func TestResolveIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		ambiguous bool
		env       string
		want      int
	}{{"explicit", false, "prod", 2}, {"ambiguous", true, "prod", 4}, {"other environment", false, "dev", 3}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nodes := []domain.Node{{ID: "arn:a", Name: "function", Kind: "lambda", Environment: "prod", Aliases: []string{"orders", "orders", ""}}, {ID: "dd:a", Name: "orders", Kind: "apm-service", Environment: tc.env}, {ID: "dd:b", Name: "database", Kind: "apm-service", Environment: tc.env}, {ID: ""}}
			if tc.ambiguous {
				nodes = append(nodes, domain.Node{ID: "arn:b", Kind: "ecs-service", Environment: "prod", Aliases: []string{"orders"}})
			}
			edge := domain.Edge{From: "dd:a", To: "dd:b", Kind: "calls"}
			g := domain.Resolve([]domain.Graph{{Nodes: nodes, Edges: []domain.Edge{edge, edge, {From: "missing", To: "dd:b"}}}})
			if len(g.Nodes) != tc.want || len(g.Edges) != 1 {
				t.Fatalf("unexpected topology: %+v", g)
			}
			if tc.want == 2 && g.Edges[0].From != "arn:a" {
				t.Fatal("edge was not redirected")
			}
			if !reflect.DeepEqual(g, domain.Resolve([]domain.Graph{g})) {
				t.Fatal("resolution is not idempotent")
			}
		})
	}
}
func TestResolvePermutation(t *testing.T) {
	t.Parallel()
	a := domain.Graph{Nodes: []domain.Node{{ID: "a", Name: "A", Aliases: []string{"x"}}}}
	b := domain.Graph{Nodes: []domain.Node{{ID: "b", Name: "B"}}, Edges: []domain.Edge{{From: "b", To: "a"}, {From: "a", To: "b"}}}
	if !reflect.DeepEqual(domain.Resolve([]domain.Graph{a, b}), domain.Resolve([]domain.Graph{b, a})) {
		t.Fatal("order changed output")
	}
}
