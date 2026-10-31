// Package s3 connects to S3-compatible object storage (ADR-0016).
package s3

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/lidisoft/horizon/internal/config"
)

// Client is an object storage client bound to the instance's bucket.
type Client struct {
	*minio.Client
	Bucket string
}

// New creates a client. It does not contact the server; use Ping.
func New(cfg config.S3) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.Endpoint.Scheme == "https",
		Region:       cfg.Region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	return &Client{Client: mc, Bucket: cfg.Bucket}, nil
}

// Ping checks that the bucket exists and the credentials can reach it.
func (c *Client) Ping(ctx context.Context) error {
	ok, err := c.BucketExists(ctx, c.Bucket)
	if err != nil {
		return fmt.Errorf("s3: %w", err)
	}
	if !ok {
		return fmt.Errorf("s3: bucket %q does not exist", c.Bucket)
	}
	return nil
}
