// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package aws

import (
	"aws-architecture-discovery/internal/domain"
	"context"
	"fmt"
	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLambdaPaginationAndMetadata(t *testing.T) {
	t.Parallel()
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/2015-03-31/functions":
			pages++
			if r.URL.Query().Get("Marker") == "next" {
				fmt.Fprint(w, `{"Functions":[]}`)
			} else {
				fmt.Fprint(w, `{"Functions":[{"FunctionName":"orders","FunctionArn":"arn:aws:lambda:us-east-1:123:function:orders"}],"NextMarker":"next"}`)
			}
		case "/2015-03-31/functions/arn:aws:lambda:us-east-1:123:function:orders/configuration":
			fmt.Fprint(w, `{"Environment":{"Variables":{"DD_SERVICE":"orders","DD_ENV":"prod","SECRET":"do-not-export"}}}`)
		case "/2015-03-31/event-source-mappings":
			fmt.Fprint(w, `{"EventSourceMappings":[{"EventSourceArn":"arn:aws:sqs:us-east-1:123:queue","FunctionArn":"arn:aws:lambda:us-east-1:123:function:orders"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	s := scan{cfg: sdk.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), BaseEndpoint: sdk.String(server.URL), HTTPClient: server.Client()}, refs: map[string][]string{}}
	if err := s.functions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(s.graph.Nodes) != 2 || len(s.graph.Edges) != 1 {
		t.Fatalf("pages=%d graph=%+v", pages, s.graph)
	}
	n := s.graph.Nodes[0]
	if n.Environment != "prod" || len(n.Aliases) != 1 || n.Aliases[0] != "orders" {
		t.Fatal("missing identity")
	}
}
func TestARNReferences(t *testing.T) {
	t.Parallel()
	id := "arn:aws:lambda:us-east-1:123:function:orders"
	if !containsARN("arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/"+id+"/invocations", id) {
		t.Fatal("integration not recognized")
	}
	if containsARN(id+"-other", id) {
		t.Fatal("partial name matched")
	}
}
func TestPlaceholderPreservesResource(t *testing.T) {
	t.Parallel()
	s := scan{graph: domain.Graph{}}
	s.add("a", "orders", "lambda")
	s.identity("a", "orders", "prod")
	s.add("a", "a", "subscription-target")
	if len(s.graph.Nodes) != 1 || s.graph.Nodes[0].Kind != "lambda" || s.graph.Nodes[0].Environment != "prod" {
		t.Fatalf("%+v", s.graph)
	}
}

func TestQueuePaginationAndURLReference(t *testing.T) {
	t.Parallel()
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		switch r.Header.Get("X-Amz-Target") {
		case "AmazonSQS.ListQueues":
			pages++
			if pages == 1 {
				fmt.Fprint(w, `{"QueueUrls":["https://sqs.us-east-1.amazonaws.com/123/jobs"],"NextToken":"page2"}`)
			} else {
				fmt.Fprint(w, `{"QueueUrls":[]}`)
			}
		case "AmazonSQS.GetQueueAttributes":
			fmt.Fprint(w, `{"Attributes":{"QueueArn":"arn:aws:sqs:us-east-1:123:jobs"}}`)
		default:
			t.Errorf("unexpected target %s", r.Header.Get("X-Amz-Target"))
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	s := scan{cfg: sdk.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), BaseEndpoint: sdk.String(server.URL), HTTPClient: server.Client()}, refs: map[string][]string{"caller": {"https://sqs.us-east-1.amazonaws.com/123/jobs"}}}
	if err := s.queues(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(s.graph.Edges) != 1 || s.graph.Edges[0].To != "arn:aws:sqs:us-east-1:123:jobs" {
		t.Fatalf("pages=%d graph=%+v", pages, s.graph)
	}
}

func TestCollectorRejectsDeniedMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		fmt.Fprint(w, `{"__type":"AccessDeniedException","message":"denied"}`)
	}))
	defer server.Close()
	s := scan{cfg: sdk.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), BaseEndpoint: sdk.String(server.URL), HTTPClient: server.Client()}}
	if err := s.functions(context.Background()); err == nil {
		t.Fatal("access denial silently ignored")
	}
}
