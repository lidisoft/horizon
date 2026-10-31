package testenv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	goruntime "runtime"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// chromeImage is the headless Chrome used by browser tests.
const chromeImage = "chromedp/headless-shell:157.0.8103.27"

// envChrome selects a local Chrome or Chromium binary instead of the container.
const envChrome = "HORIZON_TEST_CHROME"

// containerHost is how the Chrome container reaches servers on the host.
const containerHost = "host.docker.internal"

type browserEnv struct {
	alloc  context.Context
	cancel context.CancelFunc
	host   string // host name pages use to reach test servers
	bind   string // listen address of test servers
}

// startChrome starts the container (or prepares the local binary) and
// returns the containers to terminate.
func (e *env) startChrome(ctx context.Context) ([]testcontainers.Container, error) {
	if path := os.Getenv(envChrome); path != "" {
		opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(path))
		alloc, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
		e.browser = &browserEnv{alloc: alloc, cancel: cancel, host: "127.0.0.1", bind: "127.0.0.1:0"}
		return nil, nil
	}

	if goruntime.GOOS == "linux" {
		return e.startChromeHostNetwork(ctx)
	}
	return e.startChromeBridge(ctx)
}

// startChromeHostNetwork runs Chrome on the host's network (Linux), so pages
// reach test servers on 127.0.0.1 whatever the host firewall does with
// traffic from Docker bridges. DevTools listens on 127.0.0.1 only, on a free
// port, so packages testing in parallel don't collide.
func (e *env) startChromeHostNetwork(ctx context.Context) ([]testcontainers.Container, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	c, err := testcontainers.Run(ctx, chromeImage,
		// The image's script would bind DevTools on 0.0.0.0 and relay 9222.
		testcontainers.WithEntrypoint("/headless-shell/headless-shell"),
		testcontainers.WithCmd(
			"--no-sandbox", "--use-gl=angle", "--use-angle=swiftshader",
			"--remote-debugging-address=127.0.0.1",
			fmt.Sprintf("--remote-debugging-port=%d", port),
		),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.NetworkMode = "host"
		}),
		testcontainers.WithWaitStrategy(wait.ForLog("DevTools listening on")),
	)
	var containers []testcontainers.Container
	if c != nil {
		containers = append(containers, c)
	}
	if err != nil {
		return containers, err
	}
	alloc, cancel := remote.NewAllocator(context.Background(), fmt.Sprintf("http://127.0.0.1:%d", port))
	e.browser = &browserEnv{alloc: alloc, cancel: cancel, host: "127.0.0.1", bind: "127.0.0.1:0"}
	return containers, nil
}

// startChromeBridge is for Docker Desktop (macOS, Windows), where host
// networking is not the host: the container reaches test servers through
// host.docker.internal, so they listen on every interface.
func (e *env) startChromeBridge(ctx context.Context) ([]testcontainers.Container, error) {
	c, err := testcontainers.Run(ctx, chromeImage,
		testcontainers.WithExposedPorts("9222/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/json/version").WithPort("9222/tcp")),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.ExtraHosts = append(hc.ExtraHosts, containerHost+":host-gateway")
		}),
	)
	var containers []testcontainers.Container
	if c != nil {
		containers = append(containers, c)
	}
	if err != nil {
		return containers, err
	}
	endpoint, err := c.PortEndpoint(ctx, "9222/tcp", "http")
	if err != nil {
		return containers, err
	}
	alloc, cancel := remote.NewAllocator(context.Background(), endpoint)
	e.browser = &browserEnv{alloc: alloc, cancel: cancel, host: containerHost, bind: "0.0.0.0:0"}
	return containers, nil
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// Browser returns a chromedp context on a new tab, closed after the test.
func Browser(t testing.TB) context.Context {
	t.Helper()
	require(t, current != nil && current.browser != nil, "Chrome")
	ctx, cancel := chromedp.NewContext(current.browser.alloc)
	t.Cleanup(cancel)
	return ctx
}

// Serve runs h for the duration of the test and returns the base URL a page
// in Browser reaches it at, e.g. http://host.docker.internal:41234.
func Serve(t testing.TB, h http.Handler) string {
	t.Helper()
	require(t, current != nil && current.browser != nil, "Chrome")
	ln, err := net.Listen("tcp", current.browser.bind)
	if err != nil {
		t.Fatalf("testenv: listen: %v", err)
	}
	srv := &http.Server{Handler: h}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("testenv: serve: %v", err)
		}
	}()
	t.Cleanup(func() { _ = srv.Close() })
	return fmt.Sprintf("http://%s:%d", current.browser.host, ln.Addr().(*net.TCPAddr).Port)
}
