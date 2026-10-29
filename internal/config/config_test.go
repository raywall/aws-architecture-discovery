// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/raywall/aws-architecture-discovery/internal/config"
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

// TestLoadDiscoveryModes verifies YAML loading and source-specific requirements.
func TestLoadDiscoveryModes(t *testing.T) {
	// These cases isolate environment variables because Viper gives them precedence.
	for _, key := range []string{"DISCOVERY_MODE", "AWS_REGION", "OUTPUT_FORMAT", "OUTPUT_DESTINATION", "DD_API_KEY", "DD_APP_KEY", "DD_SITE", "DD_ENV_TARGET", "DISCOVERY_TIMEOUT"} {
		t.Setenv(key, "")
	}
	for _, tc := range []struct{ mode, region, want string }{
		{"AWS", "us-east-1", "aws"},
		{"DATADOG", "", "datadog"},
		{"HYBRID", "sa-east-1", "hybrid"},
		{"aws", "us-east-1", "aws"},
		{" DaTaDoG ", "", "datadog"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.yaml")
			data := "DISCOVERY_MODE: \"" + tc.mode + "\"\nAWS_REGION: \"" + tc.region + "\"\nOUTPUT_DESTINATION: ./diagrams/\nDD_API_KEY: test-key\nDD_APP_KEY: test-app\nDD_ENV_TARGET: production\n"
			if err := os.WriteFile(file, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			c, err := config.Load(file, false)
			if err != nil {
				t.Fatal(err)
			}
			if c.Mode != tc.want || c.Region != tc.region {
				t.Fatalf("mode=%q region=%q", c.Mode, c.Region)
			}
		})
	}
}

func TestModeEnvironmentOverridesYAML(t *testing.T) {
	t.Setenv("DISCOVERY_MODE", "DATADOG")
	t.Setenv("AWS_REGION", "")
	t.Setenv("DD_API_KEY", "test-key")
	t.Setenv("DD_APP_KEY", "test-app")
	t.Setenv("DD_ENV_TARGET", "production")
	t.Setenv("DD_SITE", "datadoghq.com")
	t.Setenv("OUTPUT_DESTINATION", "./diagrams/")
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("DISCOVERY_MODE: AWS\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(file, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode != "datadog" {
		t.Fatalf("mode=%q", c.Mode)
	}
}

func TestExplicitMissingFileFails(t *testing.T) {
	t.Parallel()
	if _, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"), false); err == nil {
		t.Fatal("missing config ignored")
	}
}

func TestArchitectureOutputAccepted(t *testing.T) {
	t.Parallel()
	c := config.Config{Mode: "aws", Format: "mermaid-architecture", Region: "us-east-1", Destination: "./diagrams", Timeout: time.Minute}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestIgnoreAndAccountConfiguration(t *testing.T) {
	t.Setenv("IGNORE_COMPONENTS", "")
	t.Setenv("AWS_ACCOUNT_NAME", "")
	t.Setenv("AWS_ACCOUNT_NAMES", "")
	t.Setenv("DISCOVERY_MODE", "AWS")
	t.Setenv("AWS_REGION", "us-east-1")
	file := filepath.Join(t.TempDir(), "config.yaml")
	data := "IGNORE_COMPONENTS:\n  - certificate\n  - rules\n  - ssm-parameters\nAWS_ACCOUNT_NAME: minha_conta\nAWS_ACCOUNT_NAMES:\n  '222222222222': outra_conta\n"
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(file, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.IgnoreComponents) != 3 || c.AccountName != "minha_conta" || c.AccountNames["222222222222"] != "outra_conta" {
		t.Fatal("YAML options not loaded")
	}
	t.Setenv("IGNORE_COMPONENTS", "ssm, certificate")
	t.Setenv("AWS_ACCOUNT_NAME", "override")
	c, err = config.Load(file, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.IgnoreComponents) != 2 || c.IgnoreComponents[1] != "certificate" || c.AccountName != "override" {
		t.Fatal("environment overrides not applied")
	}
}
