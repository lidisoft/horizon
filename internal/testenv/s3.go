package testenv

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/lidisoft/horizon/internal/config"
)

// S3 storage matches docker-compose.yml.
const (
	s3Image     = "rustfs/rustfs:1.0.4"
	s3AccessKey = "horizon"
	s3SecretKey = "horizon-test-secret"
)

func (e *env) startS3(ctx context.Context) (*testcontainers.DockerContainer, error) {
	c, err := testcontainers.Run(ctx, s3Image,
		testcontainers.WithEnv(map[string]string{"RUSTFS_ACCESS_KEY": s3AccessKey, "RUSTFS_SECRET_KEY": s3SecretKey}),
		testcontainers.WithExposedPorts("9000/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9000/tcp").WithStartupTimeout(time.Minute)),
	)
	if err != nil {
		return c, err
	}
	endpoint, err := c.PortEndpoint(ctx, "9000/tcp", "http")
	if err != nil {
		return c, err
	}
	if e.s3Endpoint, err = url.Parse(endpoint); err != nil {
		return c, err
	}
	// The port listens before the API answers.
	mc, err := minio.New(e.s3Endpoint.Host, &minio.Options{Creds: credentials.NewStaticV4(s3AccessKey, s3SecretKey, ""), BucketLookup: minio.BucketLookupPath})
	if err != nil {
		return c, err
	}
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		if _, err = mc.ListBuckets(ctx); err == nil || time.Now().After(deadline) {
			return c, err
		}
	}
}

// S3 creates a bucket for the test and returns its settings.
func S3(t testing.TB) config.S3 {
	t.Helper()
	require(t, current != nil && current.s3Endpoint != nil, "S3")
	cfg := config.S3{Endpoint: current.s3Endpoint, Bucket: randomName("t"), Region: config.DefaultS3Region,
		AccessKey: s3AccessKey, SecretKey: s3SecretKey}
	mc, err := minio.New(cfg.Endpoint.Host, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatalf("testenv: s3: %v", err)
	}
	if err := mc.MakeBucket(context.Background(), cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
		t.Fatalf("testenv: s3: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		for o := range mc.ListObjects(ctx, cfg.Bucket, minio.ListObjectsOptions{Recursive: true}) {
			_ = mc.RemoveObject(ctx, cfg.Bucket, o.Key, minio.RemoveObjectOptions{})
		}
		if err := mc.RemoveBucket(ctx, cfg.Bucket); err != nil {
			fmt.Println("testenv: s3: remove bucket:", err)
		}
	})
	return cfg
}
