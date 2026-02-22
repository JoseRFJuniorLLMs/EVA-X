// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	evaConfig "eva/internal/brainstem/config"
)

// BlobAdapter fornece uma interface genérica para armazenamento de objetos (S3/MinIO)
type BlobAdapter struct {
	client *s3.Client
	bucket string
}

// NewBlobAdapter inicializa o cliente S3 baseado na configuração
func NewBlobAdapter(cfg *evaConfig.Config) (*BlobAdapter, error) {
	if !cfg.S3Enabled {
		return nil, nil // Silenciosamente desabilitado
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.S3Endpoint != "" {
			return aws.Endpoint{
				URL:               cfg.S3Endpoint,
				SigningRegion:     cfg.S3Region,
				HostnameImmutable: cfg.S3ForcePathStyle,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.S3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config failed: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePathStyle
	})

	return &BlobAdapter{
		client: client,
		bucket: cfg.S3Bucket,
	}, nil
}

// Upload envia dados para o bucket
func (ba *BlobAdapter) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	if ba == nil {
		return fmt.Errorf("blob adapter not initialized")
	}

	_, err := ba.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(ba.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

// Download recupera dados do bucket
func (ba *BlobAdapter) Download(ctx context.Context, key string) ([]byte, error) {
	if ba == nil {
		return nil, fmt.Errorf("blob adapter not initialized")
	}

	resp, err := ba.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(ba.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
