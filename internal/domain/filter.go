// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package domain

import "strings"

// Exclude removes matching component types and their incident edges without modifying input.
// Selectors match a node kind, AWS service, resource type or service:resource-type.
// Filtering occurs after Resolve so an excluded resource cannot return as its APM alias.
func Exclude(g Graph, selectors []string) Graph {
	ignored := map[string]bool{}
	for _, s := range selectors {
		if s = componentName(s); s != "" {
			ignored[s] = true
		}
	}
	result := Graph{Nodes: []Node{}, Edges: []Edge{}}
	retained := map[string]bool{}
	for _, n := range g.Nodes {
		candidates := []string{n.Kind}
		arn := strings.SplitN(n.ID, ":", 6)
		if len(arn) == 6 && arn[0] == "arn" {
			candidates = append(candidates, arn[2])
			resource := strings.TrimPrefix(arn[5], "/")
			if i := strings.IndexAny(resource, "/:"); i >= 0 {
				resource = resource[:i]
				candidates = append(candidates, resource, arn[2]+":"+resource)
			}
		}
		skip := false
		for _, candidate := range candidates {
			if ignored[componentName(candidate)] {
				skip = true
				break
			}
		}
		if !skip {
			result.Nodes = append(result.Nodes, n)
			retained[n.ID] = true
		}
	}
	for _, e := range g.Edges {
		if retained[e.From] && retained[e.To] {
			result.Edges = append(result.Edges, e)
		}
	}
	return result
}

func componentName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(strings.Join(strings.Fields(s), "-"), "_", "-")
	switch s {
	case "certificates":
		return "certificate"
	case "rules":
		return "rule"
	case "parameters", "ssm-parameters", "ssm-parameter":
		return "parameter"
	}
	return s
}
