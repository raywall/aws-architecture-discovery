// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package config_test

import (
	"aws-architecture-discovery/internal/config"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*config.Config)
	}{{"mode", func(c *config.Config) { c.Mode = "oops" }}, {"format", func(c *config.Config) { c.Format = "oops" }}, {"region", func(c *config.Config) { c.Region = "" }}, {"timeout", func(c *config.Config) { c.Timeout = 0 }}, {"cloud-local", func(c *config.Config) { c.Cloud = true }}, {"destination", func(c *config.Config) { c.Destination = "s3://" }}, {"credentials", func(c *config.Config) { c.Mode = "hybrid" }}, {"site", func(c *config.Config) {
		c.Mode = "datadog"
		c.APIKey = "x"
		c.AppKey = "y"
		c.Environment = "prod"
		c.Site = "attacker.example"
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := config.Config{Mode: "aws", Format: "mermaid", Region: "us-east-1", Destination: "./out", Timeout: time.Minute}
			tc.change(&c)
			if c.Validate() == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
func TestEnvironmentPrecedence(t *testing.T) {
	t.Setenv("DISCOVERY_MODE", "aws")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("OUTPUT_FORMAT", "c4-json")
	t.Setenv("OUTPUT_DESTINATION", "./output")
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("OUTPUT_FORMAT: mermaid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(file, false)
	if err != nil || c.Format != "c4-json" {
		t.Fatalf("%+v %v", c, err)
	}
}
