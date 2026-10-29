// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package render

import (
	"bytes"
	"fmt"
	"sort"
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
	groups := map[string]string{}
	owners := map[string]string{}
	named := map[string]bool{}
	for _, n := range g.Nodes {
		account := n.AccountID
		if account == "" {
			parts := strings.SplitN(n.ID, ":", 6)
			if len(parts) == 6 && parts[0] == "arn" {
				account = parts[4]
				if account == "" {
					account = "unknown"
				}
			}
		}
		key := "runtime"
		name := "Observed services"
		if account != "" {
			key = "account:" + account
			name = n.AccountName
			if name == "" {
				name = "aws_" + account
			}
		}
		// Prefer a configured name deterministically when snapshots share an account.
		explicit := account != "" && n.AccountName != ""
		if old, ok := groups[key]; !ok || explicit && !named[key] || explicit == named[key] && name < old {
			groups[key] = name
			named[key] = explicit
		}
		owners[n.ID] = key
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ids := map[string]string{}
	counts := map[string]int{}
	for _, key := range keys {
		id := strings.ReplaceAll(architectureLabel(groups[key]), " ", "_")
		if key == "runtime" {
			id = "runtime"
		}
		ids[key] = id
		counts[id]++
	}
	for _, key := range keys {
		id := ids[key]
		if counts[id] > 1 || strings.HasPrefix(id, "n") && len(id) == 65 {
			id += "_" + identifier(key)
		}
		ids[key] = id
		fmt.Fprintf(&b, "  group %s(cloud)[%s]\n", id, architectureLabel(groups[key]))
	}
	for _, n := range g.Nodes {
		fmt.Fprintf(&b, "  service %s(%s)[%s] in %s\n", identifier(n.ID), architectureIcon(n.Kind), architectureLabel(n.Name+" - "+n.Kind), ids[owners[n.ID]])
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
