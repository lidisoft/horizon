package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	lds "github.com/lidisoft/design-system"
)

// Asset URL prefixes.
const (
	staticPrefix = "/static/"
	ldsModule    = "github.com/lidisoft/design-system"
	// specifierPrefix is the bare-specifier namespace of web/app modules.
	specifierPrefix = "horizon/"
)

// immutable is the cache policy of content-addressed URLs.
const immutable = "public, max-age=31536000, immutable"

type asset struct {
	path string // inside app/, e.g. core/api.js
	hash string
	body []byte
	ct   string
}

// Web serves the SPA: index.html for app routes, app/ files under
// /static/<hash>/<path>, and LDS under /lds/<version>/.
type Web struct {
	assets   map[string]asset // by "<hash>/<path>"
	index    []byte
	csp      string
	ldsBase  string
	ldsFiles http.Handler
}

// indexData fills web/index.html.
type indexData struct {
	// ImportMap is inserted verbatim: html/template doesn't know the
	// importmap script type and would HTML-escape it. json.Marshal already
	// escapes <, > and &, so it cannot close the script element.
	ImportMap template.HTML
	Entry     string
	LDS       string
}

// NewWeb indexes webFS (index.html and app/) and renders the shell page.
func NewWeb(webFS fs.FS) (*Web, error) {
	w := &Web{assets: map[string]asset{}}
	imports := map[string]string{}

	err := fs.WalkDir(webFS, "app", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(webFS, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		a := asset{
			path: strings.TrimPrefix(p, "app/"),
			hash: hex.EncodeToString(sum[:])[:12],
			body: body,
			ct:   contentType(p),
		}
		w.assets[a.hash+"/"+a.path] = a
		imports[specifierPrefix+a.path] = staticPrefix + a.hash + "/" + a.path
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	entry, ok := imports[specifierPrefix+"main.js"]
	if !ok {
		return nil, fmt.Errorf("web: app/main.js is missing")
	}

	importMap, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		return nil, err
	}

	version := ldsVersion()
	w.ldsBase = "/lds/" + version + "/"
	w.ldsFiles = http.StripPrefix(w.ldsBase, http.FileServer(http.FS(lds.FS())))

	tpl, err := template.ParseFS(webFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("web: index.html: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, indexData{ImportMap: template.HTML(importMap), Entry: entry, LDS: w.ldsBase}); err != nil {
		return nil, fmt.Errorf("web: index.html: %w", err)
	}
	w.index = buf.Bytes()

	// The import map is the page's only inline script: allow exactly it.
	mapHash := sha256.Sum256(importMap)
	w.csp = strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'sha256-" + base64.StdEncoding.EncodeToString(mapHash[:]) + "'",
		"style-src 'self'",
		"img-src 'self' data:",
		"font-src 'self'",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
	return w, nil
}

// ldsVersion is the design system module version from the build info, so
// LDS URLs change, and caches refresh, when go.mod moves to a new release.
func ldsVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == ldsModule {
				if dep.Replace != nil {
					return "dev"
				}
				return dep.Version
			}
		}
	}
	return "dev"
}

func contentType(p string) string {
	switch path.Ext(p) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json"
	}
	if ct := mime.TypeByExtension(path.Ext(p)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func (w *Web) register(r *gin.Engine) {
	r.GET(staticPrefix+":hash/*path", w.static)
	r.HEAD(staticPrefix+":hash/*path", w.static)
	r.GET("/lds/:version/*file", w.lds)
	r.HEAD("/lds/:version/*file", w.lds)
}

func (w *Web) static(c *gin.Context) {
	a, ok := w.assets[c.Param("hash")+c.Param("path")]
	if !ok {
		// An old hash after a deploy: the page that asked must reload.
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", immutable)
	c.Data(http.StatusOK, a.ct, a.body)
}

func (w *Web) lds(c *gin.Context) {
	if "/lds/"+c.Param("version")+"/" != w.ldsBase {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", immutable)
	w.ldsFiles.ServeHTTP(c.Writer, c.Request)
}

// serveIndex answers app routes (/, /w/acme/issues/HZN-42, …) with the shell.
func (w *Web) serveIndex(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")
	c.Header("Content-Security-Policy", w.csp)
	c.Data(http.StatusOK, "text/html; charset=utf-8", w.index)
}

// isAppRoute tells SPA routes from missing files and API paths.
func isAppRoute(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	p := r.URL.Path
	for _, prefix := range []string{"/api/", staticPrefix, "/lds/", "/swagger/"} {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	return path.Ext(p) == "" || strings.Contains(r.Header.Get("Accept"), "text/html")
}
