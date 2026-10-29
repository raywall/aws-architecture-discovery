// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package render exports deterministic Mermaid, draw.io CSV and C4-oriented JSON.
package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/raywall/aws-architecture-discovery/internal/domain"
)

// Renderer selects one artifact encoding.
type Renderer struct { // Format selects the artifact encoding.
	Format string
}

// Extension returns the output suffix.
func (r Renderer) Extension() string {
	switch r.Format {
	case "mermaid":
		return "mermaid"
	case "mermaid-architecture":
		return "architecture.mermaid"
	case "drawio-csv":
		return "csv"
	default:
		return "json"
	}
}

// ContentType returns the artifact MIME type.
func (r Renderer) ContentType() string {
	switch r.Format {
	case "c4-json":
		return "application/json"
	case "drawio-csv":
		return "text/csv; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
func identifier(s string) string { return fmt.Sprintf("n%x", sha256.Sum256([]byte(s))) }
func label(s string) string {
	s = strings.ReplaceAll(s, "&", "#38;")
	s = strings.ReplaceAll(s, "\"", "#34;")
	s = strings.ReplaceAll(s, "<", "#60;")
	s = strings.ReplaceAll(s, ">", "#62;")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// Render encodes a sorted graph; C4 JSON uses a versioned application schema, not Structurizr DSL.
func (r Renderer) Render(input domain.Graph) ([]byte, error) {
	g := domain.Resolve([]domain.Graph{input})
	var b bytes.Buffer
	switch r.Format {
	case "mermaid-architecture":
		return renderArchitecture(g), nil
	case "mermaid":
		b.WriteString("flowchart LR\n")
		for _, n := range g.Nodes {
			fmt.Fprintf(&b, "  %s[\"%s (%s)\"]\n", identifier(n.ID), label(n.Name), label(n.Kind))
		}
		for _, e := range g.Edges {
			fmt.Fprintf(&b, "  %s -->|\"%s [%s]\"| %s\n", identifier(e.From), label(e.Kind), label(e.Source), identifier(e.To))
		}
	case "drawio-csv":
		b.WriteString("# label: %name%<br>%kind%\n# identity: id\n# connect: {\"from\":\"refs\",\"to\":\"id\",\"invert\":false,\"style\":\"endArrow=classic;\"}\n# layout: auto\n")
		w := csv.NewWriter(&b)
		_ = w.Write([]string{"id", "name", "kind", "refs"})
		for _, n := range g.Nodes {
			targets := []string{}
			for _, e := range g.Edges {
				if e.From == n.ID {
					targets = append(targets, identifier(e.To))
				}
			}
			_ = w.Write([]string{identifier(n.ID), html.EscapeString(n.Name), html.EscapeString(n.Kind), strings.Join(targets, ",")})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, err
		}
	case "c4-json":
		type element struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Technology string `json:"technology"`
			Type       string `json:"type"`
		}
		type relation struct {
			Source      string `json:"source"`
			Destination string `json:"destination"`
			Description string `json:"description"`
			Evidence    string `json:"evidence"`
		}
		out := struct {
			Schema        string     `json:"schema"`
			Elements      []element  `json:"elements"`
			Relationships []relation `json:"relationships"`
		}{Schema: "github.com/raywall/aws-architecture-discovery/c4-v1", Elements: []element{}, Relationships: []relation{}}
		for _, n := range g.Nodes {
			out.Elements = append(out.Elements, element{n.ID, n.Name, n.Kind, "container"})
		}
		for _, e := range g.Edges {
			out.Relationships = append(out.Relationships, relation{e.From, e.To, e.Kind, e.Source})
		}
		return json.MarshalIndent(out, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported output format %q", r.Format)
	}
	return b.Bytes(), nil
}
