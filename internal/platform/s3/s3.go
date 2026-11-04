// Package s3 connects to S3-compatible object storage (ADR-0016).
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/lidisoft/horizon/internal/config"
)

// ErrNotFound is returned for a key with no object.
var ErrNotFound = errors.New("s3: no such object")

// Client is an object storage client bound to the instance's bucket.
type Client struct {
	*minio.Client
	Bucket string
	// public signs URLs for browsers, on the public endpoint. Signing is
	// local: it never contacts the server.
	public *minio.Client
	// PublicOrigin is the public endpoint's origin, for the page's CSP.
	PublicOrigin string
}

// New creates a client. It does not contact the server; use Ping.
func New(cfg config.S3) (*Client, error) {
	open := func(u *url.URL) (*minio.Client, error) {
		return minio.New(u.Host, &minio.Options{
			Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure:       u.Scheme == "https",
			Region:       cfg.Region,
			BucketLookup: minio.BucketLookupPath,
		})
	}
	mc, err := open(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	public := cfg.PublicEndpoint
	if public == nil {
		public = cfg.Endpoint
	}
	pc, err := open(public)
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	return &Client{Client: mc, Bucket: cfg.Bucket, public: pc, PublicOrigin: public.Scheme + "://" + public.Host}, nil
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

// AllowOrigin adds a CORS rule letting origin (the app) PUT and GET objects
// with presigned URLs, keeping the bucket's other rules.
func (c *Client) AllowOrigin(ctx context.Context, origin string) error {
	cfg, err := c.GetBucketCors(ctx, c.Bucket)
	if err != nil {
		var resp minio.ErrorResponse
		if !errors.As(err, &resp) || resp.Code != "NoSuchCORSConfiguration" {
			return fmt.Errorf("s3: cors: %w", err)
		}
	}
	var rules []cors.Rule
	if cfg != nil {
		rules = cfg.CORSRules
	}
	for _, r := range rules {
		if slices.Contains(r.AllowedOrigin, origin) && slices.Contains(r.AllowedMethod, "PUT") {
			return nil
		}
	}
	rules = append(rules, cors.Rule{
		AllowedOrigin: []string{origin}, AllowedMethod: []string{"PUT", "GET", "HEAD"},
		AllowedHeader: []string{"*"}, ExposeHeader: []string{"ETag"}, MaxAgeSeconds: 3600,
	})
	if err := c.SetBucketCors(ctx, c.Bucket, cors.NewConfig(rules)); err != nil {
		return fmt.Errorf("s3: cors: %w", err)
	}
	return nil
}

// PresignPut returns a URL the browser PUTs the object to.
func (c *Client) PresignPut(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := c.public.PresignedPutObject(ctx, c.Bucket, key, ttl)
	if err != nil {
		return "", fmt.Errorf("s3: presign: %w", err)
	}
	return u.String(), nil
}

// PresignGet returns a download URL; params set response headers
// (response-content-disposition, response-content-type).
func (c *Client) PresignGet(ctx context.Context, key string, params url.Values, ttl time.Duration) (string, error) {
	u, err := c.public.PresignedGetObject(ctx, c.Bucket, key, ttl, params)
	if err != nil {
		return "", fmt.Errorf("s3: presign: %w", err)
	}
	return u.String(), nil
}

// Stat returns an object's size.
func (c *Client) Stat(ctx context.Context, key string) (int64, error) {
	info, err := c.StatObject(ctx, c.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("s3: stat: %w", err)
	}
	return info.Size, nil
}

// ReadHead returns up to n first bytes of an object, for sniffing.
func (c *Client) ReadHead(ctx context.Context, key string, n int64) ([]byte, error) {
	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(0, n-1); err != nil {
		return nil, err
	}
	obj, err := c.GetObject(ctx, c.Bucket, key, opts)
	if err != nil {
		return nil, fmt.Errorf("s3: get: %w", err)
	}
	defer obj.Close()
	b, err := io.ReadAll(io.LimitReader(obj, n))
	if err != nil {
		return nil, fmt.Errorf("s3: get: %w", err)
	}
	return b, nil
}

// Delete removes objects; missing ones are not an error.
func (c *Client) Delete(ctx context.Context, keys []string) error {
	for _, k := range keys {
		if err := c.RemoveObject(ctx, c.Bucket, k, minio.RemoveObjectOptions{}); err != nil && minio.ToErrorResponse(err).Code != "NoSuchKey" {
			return fmt.Errorf("s3: delete %s: %w", k, err)
		}
	}
	return nil
}

// Object is a stored object, as listed.
type Object struct {
	Key      string
	Size     int64
	Modified time.Time
}

// List calls fn for every object of the bucket.
func (c *Client) List(ctx context.Context, fn func(Object) error) error {
	for o := range c.ListObjects(ctx, c.Bucket, minio.ListObjectsOptions{Recursive: true}) {
		if o.Err != nil {
			return fmt.Errorf("s3: list: %w", o.Err)
		}
		if err := fn(Object{Key: o.Key, Size: o.Size, Modified: o.LastModified}); err != nil {
			return err
		}
	}
	return nil
}
