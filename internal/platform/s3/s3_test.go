package s3_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/lidisoft/horizon/internal/platform/s3"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.S3Storage)) }

func TestPresignedRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, err := s3.New(testenv.S3(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Stat(ctx, "ws/x/att/none"); !errors.Is(err, s3.ErrNotFound) {
		t.Errorf("stat of a missing key: %v", err)
	}

	put, err := c.PresignPut(ctx, "ws/x/att/1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("hello, attachment")
	req, _ := http.NewRequest(http.MethodPut, put, bytes.NewReader(body))
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("PUT: %v %v", res, err)
	}
	if size, err := c.Stat(ctx, "ws/x/att/1"); err != nil || size != int64(len(body)) {
		t.Errorf("stat = %d, %v", size, err)
	}
	if head, err := c.ReadHead(ctx, "ws/x/att/1", 5); err != nil || string(head) != "hello" {
		t.Errorf("head = %q, %v", head, err)
	}

	params := url.Values{}
	params.Set("response-content-disposition", `attachment; filename="a.txt"`)
	get, err := c.PresignGet(ctx, "ws/x/att/1", params, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.Get(get)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(got) != string(body) || res.Header.Get("Content-Disposition") != `attachment; filename="a.txt"` {
		t.Errorf("GET = %q, %q", got, res.Header.Get("Content-Disposition"))
	}

	if err := c.Delete(ctx, []string{"ws/x/att/1", "ws/x/att/none"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Stat(ctx, "ws/x/att/1"); !errors.Is(err, s3.ErrNotFound) {
		t.Errorf("after delete: %v", err)
	}
}

func TestAllowOrigin(t *testing.T) {
	ctx := context.Background()
	c, err := s3.New(testenv.S3(t))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := c.AllowOrigin(ctx, "https://horizon.example.com"); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := c.GetBucketCors(ctx, c.Bucket)
	if err != nil || len(cfg.CORSRules) != 1 {
		t.Fatalf("cors = %+v, %v", cfg, err)
	}
	put, _ := c.PresignPut(ctx, "k", time.Minute)
	req, _ := http.NewRequest(http.MethodOptions, put, nil)
	req.Header.Set("Origin", "https://horizon.example.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.Header.Get("Access-Control-Allow-Origin") != "https://horizon.example.com" {
		t.Errorf("preflight: %d %v", res.StatusCode, res.Header)
	}
}
