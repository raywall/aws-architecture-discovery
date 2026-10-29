// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package datadog_test

import (
	"aws-architecture-discovery/internal/adapter/datadog"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDependenciesContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/service_dependencies" || r.URL.Query().Get("env") != "production" || r.Header.Get("DD-API-KEY") != "key" || r.Header.Get("DD-APPLICATION-KEY") != "app" {
			t.Error("invalid request contract")
		}
		fmt.Fprint(w, `{"orders":{"calls":["database"]},"idle":{"calls":[]}}`)
	}))
	defer server.Close()
	p := datadog.Provider{Client: server.Client(), BaseURL: server.URL, APIKey: "key", AppKey: "app", Environment: "production"}
	g, err := p.Collect(context.Background())
	if err != nil || len(g.Nodes) != 3 || len(g.Edges) != 1 {
		t.Fatalf("%+v %v", g, err)
	}
}
func TestDependenciesFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		status     int
	}{{"forbidden", "secret", 403}, {"malformed", "{", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			p := datadog.Provider{Client: server.Client(), BaseURL: server.URL}
			if _, err := p.Collect(context.Background()); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}
