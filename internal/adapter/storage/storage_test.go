// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

package storage_test

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/raywall/aws-architecture-discovery/internal/adapter/storage"
)

type client struct {
	input *s3.PutObjectInput
	data  []byte
}

func (c *client) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.input = in
	c.data, _ = io.ReadAll(in.Body)
	return &s3.PutObjectOutput{}, nil
}
func TestLocalCommit(t *testing.T) {
	t.Parallel()
	s := storage.Sink{Destination: t.TempDir()}
	location, err := s.Write(context.Background(), "architecture.mermaid", []byte("graph"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(location)
	if err != nil || string(data) != "graph" {
		t.Fatal("incorrect artifact")
	}
	info, _ := os.Stat(location)
	if info.Mode().Perm() != 0600 {
		t.Fatal("artifact is not private")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Write(ctx, "architecture.mermaid", []byte("bad"), "text/plain"); err == nil {
		t.Fatal("cancelled write succeeded")
	}
	data, _ = os.ReadFile(location)
	if string(data) != "graph" {
		t.Fatal("prior artifact overwritten")
	}
}
func TestS3Destination(t *testing.T) {
	t.Parallel()
	c := &client{}
	s := storage.Sink{Destination: "s3://bucket/maps/", Client: c}
	location, err := s.Write(context.Background(), "architecture.json", []byte("{}"), "application/json")
	if err != nil || location != "s3://bucket/maps/architecture.json" || aws.ToString(c.input.Bucket) != "bucket" || aws.ToString(c.input.Key) != "maps/architecture.json" || string(c.data) != "{}" {
		t.Fatalf("%s %v %+v", location, err, c.input)
	}
}
