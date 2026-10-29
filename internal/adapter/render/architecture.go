// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package render

import (
	"bytes"
	"fmt"
	"strings"

	"aws-architecture-discovery/internal/domain"
)

// architectureLabel keeps labels on one line and removes syntax delimiters.
// This deliberately avoids newer quoted-label syntax to support Mermaid 11.1.
func architectureLabel(value string) string {
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return ' '
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "unnamed"
	}
	return value
}

func architectureIcon(kind string) string {
	switch kind {
	case "dynamodb":
		return "database"
	case "ssm-parameter", "sqs", "kafka-topic":
		return "disk"
	case "api-gateway-rest", "api-gateway-v2", "appsync", "sns", "msk", "ecs-cluster":
		return "cloud"
	default:
		return "server"
	}
}

func renderArchitecture(g domain.Graph) []byte {
	var b bytes.Buffer
	b.WriteString("architecture-beta\n")
	hasAWS, hasRuntime := false, false
	for _, n := range g.Nodes {
		if strings.HasPrefix(n.ID, "arn:") {
			hasAWS = true
		} else {
			hasRuntime = true
		}
	}
	if hasAWS {
		b.WriteString("  group aws(cloud)[AWS resources]\n")
	}
	if hasRuntime {
		b.WriteString("  group runtime(cloud)[Observed services]\n")
	}
	for _, n := range g.Nodes {
		group := "runtime"
		if strings.HasPrefix(n.ID, "arn:") {
			group = "aws"
		}
		fmt.Fprintf(&b, "  service %s(%s)[%s] in %s\n", identifier(n.ID), architectureIcon(n.Kind), architectureLabel(n.Name+" - "+n.Kind), group)
	}
	seen := map[[2]string]bool{}
	for _, e := range g.Edges {
		// Comments retain evidence without relying on newer edge-label extensions.
		fmt.Fprintf(&b, "  %%%% %s -> %s: %s (%s)\n", identifier(e.From), identifier(e.To), architectureLabel(e.Kind), architectureLabel(e.Source))
		pair := [2]string{e.From, e.To}
		if !seen[pair] {
			fmt.Fprintf(&b, "  %s:R --> L:%s\n", identifier(e.From), identifier(e.To))
			seen[pair] = true
		}
	}
	return b.Bytes()
}
