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
	for _, format := range []string{"mermaid", "drawio-csv", "c4-json"} {
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
