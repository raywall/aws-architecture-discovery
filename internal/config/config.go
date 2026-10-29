// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package config loads and validates environment and YAML configuration.
package config

import (
	"fmt"
	"github.com/spf13/viper"
	"net/url"
	"strings"
	"time"
)

// Config contains non-exported-at-runtime settings; never log this structure because it includes secrets.
type Config struct {
	// Mode selects aws, datadog or hybrid ingestion.
	Mode string
	// Format selects the artifact encoding.
	Format string
	// Destination is the output directory or S3 prefix.
	Destination string
	// Region selects the AWS region.
	Region string
	// Profile optionally selects a local AWS profile.
	Profile string
	// Role is the optional target account role ARN.
	Role string
	// APIKey authenticates API requests and must never be logged.
	APIKey string
	// AppKey authorizes API requests and must never be logged.
	AppKey string
	// Site selects the Datadog regional API domain.
	Site string
	// Environment is the deployment environment used for identity matching.
	Environment string
	// Timeout limits the total discovery execution duration.
	Timeout time.Duration
	// Trace enables native Datadog instrumentation.
	Trace bool
	// Cloud marks execution in Lambda or ECS.
	Cloud bool
}

// Load gives environment variables precedence over an optional explicitly selected YAML file.
func Load(file string, cloud bool) (Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	defaults := map[string]interface{}{"DISCOVERY_MODE": "aws", "OUTPUT_FORMAT": "mermaid", "OUTPUT_DESTINATION": "./diagrams/", "DD_SITE": "datadoghq.com", "DISCOVERY_TIMEOUT": "10m", "DD_TRACE_ENABLED": cloud}
	for k, value := range defaults {
		v.SetDefault(k, value)
	}
	if file != "" {
		v.SetConfigFile(file)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, err
		}
	}
	c := Config{Mode: strings.ToLower(strings.TrimSpace(v.GetString("DISCOVERY_MODE"))), Format: v.GetString("OUTPUT_FORMAT"), Destination: v.GetString("OUTPUT_DESTINATION"), Region: v.GetString("AWS_REGION"), Profile: v.GetString("AWS_PROFILE"), Role: v.GetString("TARGET_ACCOUNT_ROLE_ARN"), APIKey: v.GetString("DD_API_KEY"), AppKey: v.GetString("DD_APP_KEY"), Site: v.GetString("DD_SITE"), Environment: v.GetString("DD_ENV_TARGET"), Timeout: v.GetDuration("DISCOVERY_TIMEOUT"), Trace: v.GetBool("DD_TRACE_ENABLED"), Cloud: cloud}
	return c, c.Validate()
}

// Validate rejects unsupported modes, unsafe endpoints and missing required settings.
func (c Config) Validate() error {
	if c.Mode != "aws" && c.Mode != "datadog" && c.Mode != "hybrid" {
		return fmt.Errorf("invalid DISCOVERY_MODE: use AWS, DATADOG or HYBRID")
	}
	if c.Format != "mermaid" && c.Format != "mermaid-architecture" && c.Format != "drawio-csv" && c.Format != "c4-json" {
		return fmt.Errorf("invalid OUTPUT_FORMAT")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("DISCOVERY_TIMEOUT must be positive")
	}
	if c.Destination == "" {
		return fmt.Errorf("OUTPUT_DESTINATION required")
	}
	if c.Cloud && !strings.HasPrefix(c.Destination, "s3://") {
		return fmt.Errorf("in-account execution requires an s3:// OUTPUT_DESTINATION")
	}
	if strings.Contains(c.Destination, "://") {
		u, e := url.Parse(c.Destination)
		if e != nil || u.Scheme != "s3" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid S3 destination")
		}
	}
	if (c.Mode != "datadog" || strings.HasPrefix(c.Destination, "s3://")) && c.Region == "" {
		return fmt.Errorf("AWS_REGION required")
	}
	if c.Mode != "aws" {
		if c.APIKey == "" || c.AppKey == "" || c.Environment == "" {
			return fmt.Errorf("DD_API_KEY, DD_APP_KEY and DD_ENV_TARGET required")
		}
		valid := false
		for _, s := range []string{"datadoghq.com", "datadoghq.eu", "us3.datadoghq.com", "us5.datadoghq.com", "ap1.datadoghq.com", "ap2.datadoghq.com", "uk1.datadoghq.com", "ddog-gov.com", "us2.ddog-gov.com"} {
			if c.Site == s {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("unsupported DD_SITE")
		}
	}
	return nil
}
