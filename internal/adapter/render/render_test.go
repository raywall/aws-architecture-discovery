// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package render_test

import (
	"aws-architecture-discovery/internal/adapter/render"
	"aws-architecture-discovery/internal/domain"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormats(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"mermaid", "mermaid-architecture", "drawio-csv", "c4-json"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			r := render.Renderer{Format: format}
			nodes := []domain.Node{{ID: "a", Name: "evil\"] --> X<script>\n", Kind: "lambda"}, {ID: "b", Name: "queue,one", Kind: "sqs"}}
			g := domain.Graph{Nodes: nodes, Edges: []domain.Edge{{From: "a", To: "b", Kind: "calls", Source: "aws"}}}
			data, err := r.Render(g)
			if err != nil {
				t.Fatal(err)
			}
			g.Nodes = []domain.Node{nodes[1], nodes[0]}
			again, _ := r.Render(g)
			if !bytes.Equal(data, again) {
				t.Fatal("non-deterministic output")
			}
			switch format {
			case "mermaid":
				if strings.Contains(string(data), "<script>") || strings.Contains(string(data), `evil"]`) {
					t.Fatal("unescaped label")
				}
			case "c4-json":
				if !json.Valid(data) {
					t.Fatal("invalid JSON")
				}
			case "drawio-csv":
				reader := csv.NewReader(bytes.NewReader(data))
				reader.Comment = '#'
				records, err := reader.ReadAll()
				if err != nil || len(records) != 3 {
					t.Fatalf("CSV: %v %v", records, err)
				}
			}
			if r.Extension() == "" || r.ContentType() == "" {
				t.Fatal("missing metadata")
			}
		})
	}
	if _, err := (render.Renderer{Format: "unknown"}).Render(domain.Graph{}); err == nil {
		t.Fatal("unsupported format accepted")
	}
}

func TestArchitectureServicesAndEvidence(t *testing.T) {
	t.Parallel()
	g := domain.Graph{
		Nodes: []domain.Node{
			{ID: "arn:aws:lambda:us-east-1:123:function:orders", Name: "orders", Kind: "lambda"},
			{ID: "arn:aws:dynamodb:us-east-1:123:table/orders", Name: "Orders", Kind: "dynamodb"},
			{ID: "datadog:prod:web", Name: "web]\nservice injected(server)[bad", Kind: "apm-service"},
		},
		Edges: []domain.Edge{
			{From: "datadog:prod:web", To: "arn:aws:lambda:us-east-1:123:function:orders", Kind: "calls", Source: "datadog"},
			{From: "datadog:prod:web", To: "arn:aws:lambda:us-east-1:123:function:orders", Kind: "references", Source: "aws"},
		},
	}
	r := render.Renderer{Format: "mermaid-architecture"}
	data, err := r.Render(g)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, part := range []string{"architecture-beta\n", "group aws(cloud)", "group runtime(cloud)", "(database)[Orders dynamodb]", "calls (datadog)", "references (aws)"} {
		if !strings.Contains(text, part) {
			t.Errorf("missing %q", part)
		}
	}
	if strings.Count(text, "\n  service ") != 3 || strings.Count(text, ":R --> L:") != 1 || strings.Contains(text, "\nservice injected") {
		t.Fatalf("unexpected services or edges: %s", text)
	}
	if r.Extension() != "architecture.mermaid" {
		t.Fatal("architecture artifact would overwrite flowchart")
	}
}
