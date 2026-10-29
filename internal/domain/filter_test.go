// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package domain_test

import (
	"reflect"
	"testing"

	"github.com/raywall/aws-architecture-discovery/internal/domain"
)

func TestExcludeComponents(t *testing.T) {
	t.Parallel()
	g := domain.Graph{Nodes: []domain.Node{
		{ID: "arn:aws:acm:r:123:certificate/id", Kind: "acm"},
		{ID: "arn:aws:events:r:123:rule/name", Kind: "events"},
		{ID: "arn:aws:ssm:r:123:parameter/password", Kind: "ssm-parameter"},
		{ID: "arn:aws:ssm:r:123:document/name", Kind: "ssm"},
		{ID: "lambda", Kind: "lambda"},
	}, Edges: []domain.Edge{{From: "lambda", To: "arn:aws:ssm:r:123:parameter/password"}, {From: "lambda", To: "arn:aws:ssm:r:123:document/name"}}}
	for _, tc := range []struct {
		name         string
		selectors    []string
		nodes, edges int
	}{
		{"disabled", nil, 5, 2},
		{"examples", []string{"Certificate", " RULES ", "SSM parameters"}, 2, 1},
		{"entire service", []string{"ssm"}, 3, 0},
		{"qualified type", []string{"events:rule"}, 4, 2},
		{"unknown does not match names", []string{"password"}, 5, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := domain.Exclude(g, tc.selectors)
			if len(got.Nodes) != tc.nodes || len(got.Edges) != tc.edges {
				t.Fatalf("got %d nodes %d edges", len(got.Nodes), len(got.Edges))
			}
		})
	}
	if len(g.Nodes) != 5 || len(g.Edges) != 2 {
		t.Fatal("input mutated")
	}
}

func TestExcludedAWSIdentityDoesNotReturnAsAPM(t *testing.T) {
	t.Parallel()
	resolved := domain.Resolve([]domain.Graph{{Nodes: []domain.Node{{ID: "arn:aws:lambda:r:123:function:a", Kind: "lambda", Aliases: []string{"a"}, Environment: "prod"}, {ID: "dd:a", Name: "a", Kind: "apm-service", Environment: "prod"}}}})
	got := domain.Exclude(resolved, []string{"lambda"})
	if !reflect.DeepEqual(got, domain.Graph{Nodes: []domain.Node{}, Edges: []domain.Edge{}}) {
		t.Fatalf("excluded identity remains: %+v", got)
	}
}
