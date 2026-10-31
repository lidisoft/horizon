package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/web"
)

func webEngine(t *testing.T) (http.Handler, *Web) {
	t.Helper()
	w, err := NewWeb(web.FS())
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(Deps{Log: zerolog.Nop(), Web: w}), w
}

var importMapRe = regexp.MustCompile(`<script type="importmap">(.*?)</script>`)

func TestIndexCarriesImportMapAllowedByCSP(t *testing.T) {
	h, w := webEngine(t)
	rec := get(t, h, "/w/acme/issues/HZN-42", http.Header{"Accept": {"text/html"}})
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("got %d, cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	body := rec.Body.String()

	m := importMapRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no import map in index.html")
	}
	sum := sha256.Sum256([]byte(m[1]))
	hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' "+hash) || strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP %q does not allow exactly the import map %s", csp, hash)
	}

	var imports struct{ Imports map[string]string }
	if err := json.Unmarshal([]byte(m[1]), &imports); err != nil {
		t.Fatal(err)
	}
	entry := imports.Imports["horizon/main.js"]
	if !strings.HasPrefix(entry, "/static/") || !strings.Contains(body, `<script type="module" src="`+entry+`">`) {
		t.Errorf("entry %q not loaded by index.html", entry)
	}
	if !strings.Contains(body, `href="`+w.ldsBase+`lds.min.css"`) || !strings.Contains(body, `src="`+w.ldsBase+`lds.min.js" defer`) {
		t.Errorf("index.html does not load LDS from %s", w.ldsBase)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
}

func TestStaticAssetsAreContentAddressed(t *testing.T) {
	h, w := webEngine(t)
	var url string
	for _, a := range w.assets {
		if a.path == "core/api.js" {
			url = staticPrefix + a.hash + "/" + a.path
		}
	}
	rec := get(t, h, url, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != immutable ||
		rec.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("%s: %d %q %q", url, rec.Code, rec.Header().Get("Cache-Control"), rec.Header().Get("Content-Type"))
	}
	if rec := get(t, h, "/static/000000000000/core/api.js", nil); rec.Code != http.StatusNotFound {
		t.Errorf("stale hash: got %d", rec.Code)
	}
}

func TestLDSIsServedAtItsVersion(t *testing.T) {
	h, w := webEngine(t)
	rec := get(t, h, w.ldsBase+"lds.min.css", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != immutable {
		t.Fatalf("%slds.min.css: %d", w.ldsBase, rec.Code)
	}
	if rec := get(t, h, w.ldsBase+"fonts/Inter-Variable.woff2", nil); rec.Code != http.StatusOK {
		t.Errorf("font: %d", rec.Code)
	}
	if rec := get(t, h, "/lds/v0.0.0-other/lds.min.css", nil); rec.Code != http.StatusNotFound {
		t.Errorf("other version: %d", rec.Code)
	}
}

func TestFallbacks(t *testing.T) {
	h, _ := webEngine(t)
	rec := get(t, h, "/api/v1/nothing", nil)
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/problem+json" ||
		!strings.Contains(rec.Body.String(), `"title":"Not Found"`) {
		t.Errorf("api 404: %d %q %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	if rec := get(t, h, "/favicon.ico", nil); rec.Code != http.StatusNotFound {
		t.Errorf("missing file: %d", rec.Code)
	}
	if rec := get(t, h, "/", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<hzn-app>") {
		t.Errorf("root: %d", rec.Code)
	}
}

// Source rules for web/app (docs/frontend.md).
var (
	importRe = regexp.MustCompile(`(?m)(?:^\s*import\s[^"']*?|^\s*export\s[^"']*?\sfrom\s|import\()\s*["']([^"']+)["']`)
	tKeyRe   = regexp.MustCompile(`\bt\("([^"]+)"`)
)

func TestModulesUseResolvableBareSpecifiers(t *testing.T) {
	files := map[string]string{}
	err := fs.WalkDir(web.FS(), "app", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(web.FS(), p)
		files[strings.TrimPrefix(p, "app/")] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	var catalog map[string]any
	if err := json.Unmarshal([]byte(files["i18n/en.json"]), &catalog); err != nil {
		t.Fatalf("en.json: %v", err)
	}

	for name, src := range files {
		if !strings.HasSuffix(name, ".js") {
			continue
		}
		for _, m := range importRe.FindAllStringSubmatch(src, -1) {
			spec := m[1]
			if !strings.HasPrefix(spec, specifierPrefix) {
				t.Errorf("%s imports %q: use a horizon/… specifier, so the import map can hash it", name, spec)
				continue
			}
			if _, ok := files[strings.TrimPrefix(spec, specifierPrefix)]; !ok {
				t.Errorf("%s imports %q: no such file in web/app", name, spec)
			}
		}
		for _, m := range tKeyRe.FindAllStringSubmatch(src, -1) {
			if _, ok := catalog[m[1]]; !ok {
				t.Errorf("%s uses message %q, missing from i18n/en.json", name, m[1])
			}
		}
	}
}
