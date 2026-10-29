// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package main wires discovery for CLI, ECS tasks and the Lambda runtime.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	awsadapter "github.com/raywall/aws-architecture-discovery/internal/adapter/aws"
	"github.com/raywall/aws-architecture-discovery/internal/adapter/datadog"
	"github.com/raywall/aws-architecture-discovery/internal/adapter/render"
	"github.com/raywall/aws-architecture-discovery/internal/adapter/storage"
	"github.com/raywall/aws-architecture-discovery/internal/adapter/telemetry"
	"github.com/raywall/aws-architecture-discovery/internal/application"
	"github.com/raywall/aws-architecture-discovery/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("service", "github.com/raywall/aws-architecture-discovery"))
	if err := run(); err != nil {
		slog.Error("discovery failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	file := flag.String("config", "", "optional YAML configuration file")
	flag.Parse()
	isLambda := os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
	cloud := isLambda || os.Getenv("ECS_CONTAINER_METADATA_URI_V4") != "" || os.Getenv("ECS_CONTAINER_METADATA_URI") != "" || strings.HasPrefix(os.Getenv("AWS_EXECUTION_ENV"), "AWS_ECS")
	c, err := config.Load(*file, cloud)
	if err != nil {
		return err
	}
	if c.Trace {
		if err = tracer.Start(tracer.WithService("github.com/raywall/aws-architecture-discovery")); err != nil {
			return err
		}
		defer tracer.Stop()
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	app := application.Discovery{IgnoreComponents: c.IgnoreComponents, Renderer: render.Renderer{Format: c.Format}}
	sink := storage.Sink{Destination: c.Destination}
	if c.Mode != "datadog" || strings.HasPrefix(c.Destination, "s3://") {
		options := []func(*awsconfig.LoadOptions) error{awsconfig.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}), awsconfig.WithRetryMaxAttempts(3)}
		if c.Region != "" {
			options = append(options, awsconfig.WithRegion(c.Region))
		}
		if c.Profile != "" && !cloud {
			options = append(options, awsconfig.WithSharedConfigProfile(c.Profile))
		}
		base, err := awsconfig.LoadDefaultConfig(initCtx, options...)
		if err != nil {
			return err
		}
		// Artifact writes use the execution identity, while discovery may assume a target account role.
		if strings.HasPrefix(c.Destination, "s3://") {
			sink.Client = s3.NewFromConfig(base)
		}
		if c.Mode != "datadog" {
			target := base
			if c.Role != "" {
				target.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(base), c.Role))
			}
			app.Providers = append(app.Providers, telemetry.Provider{Name: "aws", Inner: awsadapter.Provider{Config: target, AccountName: c.AccountName, AccountNames: c.AccountNames}})
		}
	}
	if c.Mode != "aws" {
		app.Providers = append(app.Providers, telemetry.Provider{Name: "datadog", Inner: datadog.Provider{Client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, BaseURL: "https://api." + c.Site, APIKey: c.APIKey, AppKey: c.AppKey, Environment: c.Environment}})
	}
	app.Sink = sink
	execute := func(parent context.Context) (location string, err error) {
		ctx, cancel := context.WithTimeout(parent, c.Timeout)
		defer cancel()
		span, ctx := tracer.StartSpanFromContext(ctx, "discovery.execute")
		defer func() {
			span.Finish(tracer.WithError(err))
			if isLambda && c.Trace {
				tracer.Flush()
			}
		}()
		location, err = app.Execute(ctx)
		if err == nil {
			slog.InfoContext(ctx, "artifact generated", "location", location, "dd.trace_id", span.Context().TraceID(), "dd.span_id", span.Context().SpanID())
		}
		return
	}
	if isLambda {
		lambda.Start(func(ctx context.Context, event json.RawMessage) (events.APIGatewayProxyResponse, error) {
			location, err := execute(ctx)
			if err != nil {
				return events.APIGatewayProxyResponse{}, err
			}
			body, err := json.Marshal(map[string]string{"location": location})
			return events.APIGatewayProxyResponse{StatusCode: 200, Headers: map[string]string{"Content-Type": "application/json"}, Body: string(body)}, err
		})
		return nil
	}
	_, err = execute(ctx)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}
