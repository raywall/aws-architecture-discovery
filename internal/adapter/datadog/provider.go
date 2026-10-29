// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package datadog reads the documented APM service dependencies API.
package datadog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/raywall/aws-architecture-discovery/internal/domain"
)

// Provider retrieves service-level call relationships; it never invents operation-level data.
type Provider struct {
	// Client performs outbound requests through an injectable client.
	Client *http.Client
	// BaseURL selects the Datadog API origin.
	BaseURL string
	// APIKey authenticates API requests and must never be logged.
	APIKey string
	// AppKey authorizes API requests and must never be logged.
	AppKey string
	// Environment is the deployment environment used for identity matching.
	Environment string
}

// Collect retrieves the environment's last hour of observed dependencies.
func (p Provider) Collect(ctx context.Context) (domain.Graph, error) {
	u, err := url.Parse(p.BaseURL + "/api/v1/service_dependencies")
	if err != nil {
		return domain.Graph{}, err
	}
	q := u.Query()
	q.Set("env", p.Environment)
	u.RawQuery = q.Encode()
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return domain.Graph{}, err
		}
		req.Header.Set("DD-API-KEY", p.APIKey)
		req.Header.Set("DD-APPLICATION-KEY", p.AppKey)
		resp, err := p.Client.Do(req)
		if err != nil {
			return domain.Graph{}, fmt.Errorf("datadog request failed: %w", err)
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt == 2 {
				return domain.Graph{}, fmt.Errorf("datadog HTTP %d after retries", resp.StatusCode)
			}
			timer := time.NewTimer(time.Duration(1<<attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return domain.Graph{}, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return domain.Graph{}, fmt.Errorf("datadog HTTP %d", resp.StatusCode)
		}
		var payload map[string]struct {
			Calls []string `json:"calls"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return domain.Graph{}, fmt.Errorf("decode dependencies: %w", err)
		}
		g := domain.Graph{}
		seen := map[string]bool{}
		id := func(s string) string { return "datadog:" + p.Environment + ":" + s }
		add := func(s string) {
			if !seen[s] {
				g.Nodes = append(g.Nodes, domain.Node{ID: id(s), Name: s, Kind: "apm-service", Environment: p.Environment})
				seen[s] = true
			}
		}
		for name, entry := range payload {
			add(name)
			for _, target := range entry.Calls {
				add(target)
				g.Edges = append(g.Edges, domain.Edge{From: id(name), To: id(target), Kind: "calls", Source: "datadog"})
			}
		}
		return g, nil
	}
	return domain.Graph{}, fmt.Errorf("datadog retries exhausted")
}
