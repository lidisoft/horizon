package testenv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"golang.org/x/crypto/bcrypt"
)

// dexImage is the OpenID Connect provider the SSO tests use.
const dexImage = "ghcr.io/dexidp/dex:v2.46.1"

// DexUser is an account of the test provider.
type DexUser struct {
	Email, Name, Password, Subject string
}

// DexUsers are the provider's accounts: their emails reach the domains
// tests allow or refuse.
var DexUsers = []DexUser{
	{Email: "grace@example.com", Name: "Grace Hopper", Password: "dex-password-grace", Subject: "grace"},
	{Email: "alan@acme.example", Name: "Alan Turing", Password: "dex-password-alan", Subject: "alan"},
}

// DexSlugs are the provider slugs whose callbacks the client allows.
var DexSlugs = []string{"dex", "acme-dex"}

// OIDCEnv is a running Dex with one confidential client, and an address
// reserved for the app it redirects back to.
type OIDCEnv struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// AppURL is where tests serve the app (Serve): the client's redirect
	// URIs are AppURL/api/v1/auth/oidc/{slug}/callback for DexSlugs.
	AppURL  string
	handler atomic.Pointer[http.Handler]
}

// OIDC returns the package's Dex.
func OIDC(t testing.TB) *OIDCEnv {
	t.Helper()
	require(t, current != nil && current.dex != nil, "Dex")
	return current.dex
}

// Serve serves h at AppURL for the rest of the test.
func (o *OIDCEnv) Serve(t testing.TB, h http.Handler) {
	o.handler.Store(&h)
	t.Cleanup(func() {
		none := http.NotFoundHandler()
		o.handler.Store(&none)
	})
}

// startDex runs Dex with an in-memory store, the password connector and
// the users above, and serves the reserved app address.
func (e *env) startDex(ctx context.Context) ([]testcontainers.Container, error) {
	port, err := freePort(ctx)
	if err != nil {
		return nil, err
	}
	app, err := new(net.ListenConfig).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	o := &OIDCEnv{
		Issuer:   fmt.Sprintf("http://127.0.0.1:%d/dex", port),
		ClientID: "horizon", ClientSecret: "horizon-dex-secret",
		AppURL: fmt.Sprintf("http://127.0.0.1:%d", app.Addr().(*net.TCPAddr).Port),
	}
	none := http.NotFoundHandler()
	o.handler.Store(&none)
	go func() {
		_ = http.Serve(app, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:gosec // test server
			(*o.handler.Load()).ServeHTTP(w, r)
		}))
	}()

	var cfg strings.Builder
	listen := "127.0.0.1"
	if goruntime.GOOS != "linux" {
		listen = "0.0.0.0"
	}
	fmt.Fprintf(&cfg, "issuer: %s\nstorage:\n  type: memory\nweb:\n  http: %s:%d\noauth2:\n  skipApprovalScreen: true\n", o.Issuer, listen, port)
	fmt.Fprintf(&cfg, "staticClients:\n- id: %s\n  secret: %s\n  name: Horizon\n  redirectURIs:\n", o.ClientID, o.ClientSecret)
	for _, slug := range DexSlugs {
		fmt.Fprintf(&cfg, "  - %s/api/v1/auth/oidc/%s/callback\n", o.AppURL, slug)
	}
	cfg.WriteString("enablePasswordDB: true\nstaticPasswords:\n")
	for _, u := range DexUsers {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&cfg, "- email: %q\n  hash: %q\n  username: %q\n  userID: %q\n", u.Email, hash, u.Name, u.Subject)
	}

	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithFiles(testcontainers.ContainerFile{Reader: strings.NewReader(cfg.String()), ContainerFilePath: "/etc/dex/horizon.yaml", FileMode: 0o644}),
		testcontainers.WithCmd("dex", "serve", "/etc/dex/horizon.yaml"),
	}
	if goruntime.GOOS == "linux" {
		opts = append(opts, testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) { hc.NetworkMode = "host" }))
	} else {
		p := strconv.Itoa(port) + "/tcp"
		opts = append(opts, testcontainers.WithExposedPorts(p), testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{network.MustParsePort(p): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: strconv.Itoa(port)}}}
		}))
	}
	c, err := testcontainers.Run(ctx, dexImage, opts...)
	var containers []testcontainers.Container
	if c != nil {
		containers = append(containers, c)
	}
	if err != nil {
		return containers, err
	}
	if err := waitHTTP(ctx, o.Issuer+"/.well-known/openid-configuration"); err != nil {
		return containers, err
	}
	e.dex = o
	return containers, nil
}
