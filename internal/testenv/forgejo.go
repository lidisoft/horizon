package testenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	goruntime "runtime"
	"strconv"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
)

// forgejoImage is the Forgejo the Git integration tests use.
const forgejoImage = "codeberg.org/forgejo/forgejo:17.0.1-rootless"

// ForgeEnv is a running Forgejo with an admin account and its API token.
type ForgeEnv struct {
	// URL is the forge's address, from the tests and from itself.
	URL      string
	User     string
	Password string
	Token    string
	host     string // how the forge reaches servers of the tests
	bind     string // listen address of those servers
}

// Forge returns the package's Forgejo.
func Forge(t testing.TB) *ForgeEnv {
	t.Helper()
	require(t, current != nil && current.forge != nil, "Forgejo")
	return current.forge
}

// ListenForForge reserves an address the forge reaches (its webhooks), and
// returns the listener and the URL the forge should use.
func (f *ForgeEnv) ListenForForge(t testing.TB) (net.Listener, string) {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", f.bind)
	if err != nil {
		t.Fatalf("testenv: listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln, fmt.Sprintf("http://%s:%d", f.host, ln.Addr().(*net.TCPAddr).Port)
}

// API calls the forge's REST API as the admin; out, if not nil, takes the
// JSON answer. It fails the test on any status but 2xx.
func (f *ForgeEnv) API(t testing.TB, method, path string, in, out any) {
	t.Helper()
	if err := f.call(t.Context(), method, path, in, out); err != nil {
		t.Fatal(err)
	}
}

func (f *ForgeEnv) call(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, f.URL+"/api/v1"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if f.Token != "" {
		req.Header.Set("Authorization", "token "+f.Token)
	} else {
		req.SetBasicAuth(f.User, f.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("forgejo: %s %s: %d %s", method, path, resp.StatusCode, b)
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

// startForgejo runs Forgejo (on the host's network on Linux, like Chrome,
// so its webhooks reach test servers on 127.0.0.1), then creates the admin
// and a token.
func (e *env) startForgejo(ctx context.Context) ([]testcontainers.Container, error) {
	f := &ForgeEnv{User: "horizon", Password: "Horizon-test-1234"}
	env := map[string]string{
		"FORGEJO__security__INSTALL_LOCK":        "true",
		"FORGEJO__database__DB_TYPE":             "sqlite3",
		"FORGEJO__webhook__ALLOWED_HOST_LIST":    "*",
		"FORGEJO__webhook__DELIVER_TIMEOUT":      "10",
		"FORGEJO__server__DISABLE_SSH":           "true",
		"FORGEJO__server__START_SSH_SERVER":      "false",
		"FORGEJO__service__DISABLE_REGISTRATION": "true",
		"FORGEJO__log__LEVEL":                    "Warn",
	}
	opts := []testcontainers.ContainerCustomizer{}
	if goruntime.GOOS == "linux" {
		port, err := freePort(ctx)
		if err != nil {
			return nil, err
		}
		f.URL, f.host, f.bind = fmt.Sprintf("http://127.0.0.1:%d", port), "127.0.0.1", "127.0.0.1:0"
		env["FORGEJO__server__HTTP_ADDR"] = "127.0.0.1"
		env["FORGEJO__server__HTTP_PORT"] = strconv.Itoa(port)
		env["FORGEJO__server__ROOT_URL"] = f.URL + "/"
		opts = append(opts, testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) { hc.NetworkMode = "host" }))
	} else {
		f.host, f.bind = containerHost, "0.0.0.0:0"
		opts = append(opts, testcontainers.WithExposedPorts("3000/tcp"),
			testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
				hc.ExtraHosts = append(hc.ExtraHosts, containerHost+":host-gateway")
			}))
	}
	opts = append(opts, testcontainers.WithEnv(env)) // ready when healthy (below)
	c, err := testcontainers.Run(ctx, forgejoImage, opts...)
	var containers []testcontainers.Container
	if c != nil {
		containers = append(containers, c)
	}
	if err != nil {
		return containers, err
	}
	if f.URL == "" {
		if f.URL, err = c.PortEndpoint(ctx, "3000/tcp", "http"); err != nil {
			return containers, err
		}
	}
	if err := waitHTTP(ctx, f.URL+"/api/healthz"); err != nil {
		return containers, err
	}
	code, out, err := c.Exec(ctx, []string{"forgejo", "admin", "user", "create", "--admin", "--username", f.User,
		"--password", f.Password, "--email", "horizon@example.com", "--must-change-password=false"})
	if err != nil {
		return containers, err
	}
	if code != 0 {
		b, _ := io.ReadAll(out)
		return containers, fmt.Errorf("create admin: %s", b)
	}
	var tok struct {
		SHA1 string `json:"sha1"`
	}
	if err := f.call(ctx, http.MethodPost, "/users/"+f.User+"/tokens", map[string]any{
		"name": "horizon-tests", "scopes": []string{"write:repository", "write:issue", "write:user"},
	}, &tok); err != nil {
		return containers, err
	}
	f.Token = tok.SHA1
	e.forge = f
	return containers, nil
}

func waitHTTP(ctx context.Context, url string) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("not healthy: " + url)
}
