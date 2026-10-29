// Copyright (c) 2026 aws-architecture-discovery contributors.
// Author: aws-architecture-discovery contributors.
// Created: 2026-10-29.

// Package storage persists artifacts atomically to disk or with an S3 PutObject.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// S3Client is the minimal output API needed by Sink.
type S3Client interface {
	// PutObject defines the PutObject operation contract.
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// Sink routes output to a directory or S3 prefix.
type Sink struct {
	// Destination is the output directory or S3 prefix.
	Destination string
	// Client performs outbound requests through an injectable client.
	Client S3Client
}

// Write commits a complete artifact; local files are private and renamed atomically.
func (s Sink) Write(ctx context.Context, name string, data []byte, contentType string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.HasPrefix(s.Destination, "s3://") {
		u, err := url.Parse(s.Destination)
		if err != nil {
			return "", err
		}
		key := path.Join(strings.TrimPrefix(u.Path, "/"), name)
		if s.Client == nil {
			return "", fmt.Errorf("S3 client required")
		}
		_, err = s.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(u.Host), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String(contentType)})
		return "s3://" + u.Host + "/" + key, err
	}
	if err := os.MkdirAll(s.Destination, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(s.Destination, ".discovery-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	target := filepath.Join(s.Destination, name)
	return target, os.Rename(f.Name(), target)
}
