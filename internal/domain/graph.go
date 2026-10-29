// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package domain resolves infrastructure and runtime topology without external dependencies.
package domain

import (
	"sort"
	"strings"
)

// Node is a resource or an observed service. Aliases are explicit identity evidence.
type Node struct {
	// AccountID identifies the owning AWS account, including resources whose ARN omits it.
	AccountID string
	// AccountName is the configured display name of the owning account.
	AccountName string
	// ID is the stable resource identity.
	ID string
	// Name is the human-readable resource label.
	Name string
	// Kind identifies the resource or relationship type.
	Kind string
	// Environment is the deployment environment used for identity matching.
	Environment string
	// Aliases contains explicit APM service identity evidence.
	Aliases []string
}

// Edge represents a directed relationship with its evidence source.
type Edge struct {
	// From identifies the source node.
	From string
	// To identifies the destination node.
	To string
	// Kind identifies the resource or relationship type.
	Kind string
	// Source identifies the collector that observed the relationship.
	Source string
}

// Graph is a snapshot of discovered topology, owned by one execution.
type Graph struct {
	// Nodes contains discovered resources and services.
	Nodes []Node
	// Edges contains directed topology relationships.
	Edges []Edge
}

// Resolve merges runtime nodes only when an alias identifies exactly one structural node.
// It preserves ambiguous identities and sorts and deduplicates the final snapshot.
func Resolve(parts []Graph) Graph {
	nodes := map[string]Node{}
	var edges []Edge
	for _, p := range parts {
		for _, n := range p.Nodes {
			if n.ID == "" {
				continue
			}
			if old, ok := nodes[n.ID]; ok {
				n.Aliases = append(n.Aliases, old.Aliases...)
			}
			nodes[n.ID] = n
		}
		edges = append(edges, p.Edges...)
	}
	aliases := map[string]map[string]bool{}
	for id, n := range nodes {
		if n.Kind == "apm-service" {
			continue
		}
		for _, a := range n.Aliases {
			if a == "" {
				continue
			}
			key := n.Environment + "\x00" + a
			if aliases[key] == nil {
				aliases[key] = map[string]bool{}
			}
			aliases[key][id] = true
		}
	}
	redirects := map[string]string{}
	for id, n := range nodes {
		if n.Kind != "apm-service" {
			continue
		}
		matches := aliases[n.Environment+"\x00"+n.Name]
		if len(matches) == 1 {
			for target := range matches {
				redirects[id] = target
			}
			delete(nodes, id)
		}
	}
	result := Graph{Nodes: []Node{}, Edges: []Edge{}}
	for _, n := range nodes {
		n.Aliases = unique(append([]string(nil), n.Aliases...))
		result.Nodes = append(result.Nodes, n)
	}
	seen := map[Edge]bool{}
	for _, e := range edges {
		if v, ok := redirects[e.From]; ok {
			e.From = v
		}
		if v, ok := redirects[e.To]; ok {
			e.To = v
		}
		if _, ok := nodes[e.From]; !ok {
			continue
		}
		if _, ok := nodes[e.To]; !ok {
			continue
		}
		if !seen[e] {
			result.Edges = append(result.Edges, e)
			seen[e] = true
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	sort.Slice(result.Edges, func(i, j int) bool {
		a, b := result.Edges[i], result.Edges[j]
		return strings.Join([]string{a.From, a.To, a.Kind, a.Source}, "\x00") < strings.Join([]string{b.From, b.To, b.Kind, b.Source}, "\x00")
	})
	return result
}
func unique(v []string) []string {
	sort.Strings(v)
	r := []string{}
	for _, s := range v {
		if s != "" && (len(r) == 0 || r[len(r)-1] != s) {
			r = append(r, s)
		}
	}
	return r
}
