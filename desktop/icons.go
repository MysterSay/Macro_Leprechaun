package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var iconCache sync.Map

func iconURLAllowed(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch u.Hostname() {
	case "api.iconify.design", "api.svgl.app", "svgl.app", "www.vectorlogo.zone", "vectorlogo.zone", "cdn.simpleicons.org", "simpleicons.org", "www.simpleicons.org", "devicon.dev":
		return true
	case "cdn.jsdelivr.net":
		return strings.HasPrefix(u.Path, "/npm/simple-icons") || strings.HasPrefix(u.Path, "/gh/devicons/devicon")
	case "raw.githubusercontent.com":
		return strings.HasPrefix(u.Path, "/simple-icons/simple-icons/") || strings.HasPrefix(u.Path, "/devicons/devicon/") || strings.HasPrefix(u.Path, "/vectorlogozone/vectorlogozone/") || strings.HasPrefix(u.Path, "/pheralb/svgl/")
	}
	return false
}
func fetchIconResource(raw string) ([]byte, error) {
	if !iconURLAllowed(raw) {
		return nil, fmt.Errorf("Use a direct HTTPS asset URL from a supported icon source")
	}
	if cached, ok := iconCache.Load(raw); ok {
		v := cached.(struct {
			B  []byte
			At time.Time
		})
		if time.Since(v.At) < 5*time.Minute {
			return v.B, nil
		}
	}
	client := http.Client{Timeout: 12 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 4 || !iconURLAllowed(r.URL.String()) {
			return fmt.Errorf("unsupported redirect")
		}
		return nil
	}}
	r, e := client.Get(raw)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("Icon source: HTTP %d", r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 2<<20 {
		return nil, fmt.Errorf("asset exceeds 2 MiB")
	}
	if count := 0; true {
		iconCache.Range(func(k, v any) bool { count++; return true })
		if count > 128 {
			iconCache.Range(func(k, v any) bool { iconCache.Delete(k); return true })
		}
	}
	iconCache.Store(raw, struct {
		B  []byte
		At time.Time
	}{b, time.Now()})
	return b, nil
}
func validateSVG(b []byte) error {
	d := xml.NewDecoder(bytes.NewReader(b))
	depth := 0
	root := false
	for {
		v, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		switch t := v.(type) {
		case xml.StartElement:
			if root && depth == 0 {
				return fmt.Errorf("multiple SVG roots")
			}
			if !root {
				if t.Name.Local != "svg" {
					return fmt.Errorf("not SVG")
				}
				root = true
			}
			depth++
			tag := strings.ToLower(t.Name.Local)
			if tag == "script" || tag == "foreignobject" || tag == "iframe" || tag == "object" || tag == "embed" || tag == "animate" || tag == "set" || tag == "animatetransform" {
				return fmt.Errorf("unsupported active SVG content")
			}
			for _, a := range t.Attr {
				key, val := strings.ToLower(a.Name.Local), strings.TrimSpace(a.Value)
				if strings.HasPrefix(key, "on") || ((key == "href" || key == "src") && !strings.HasPrefix(val, "#")) {
					return fmt.Errorf("external SVG content is not supported")
				}
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return fmt.Errorf("SVG directives are not supported")
		}
	}
	lower := strings.ToLower(string(b))
	if !root || depth != 0 || strings.Contains(lower, "@import") || strings.Contains(lower, "javascript:") || regexp.MustCompile(`url\(\s*["']?(https?:|//|data:)`).MatchString(lower) {
		return fmt.Errorf("unsupported SVG content")
	}
	return nil
}
func mediaExtension(b []byte) (string, error) {
	trim := bytes.TrimSpace(b)
	if bytes.HasPrefix(trim, []byte("<")) {
		return ".svg", validateSVG(b)
	}
	typ := http.DetectContentType(b)
	switch typ {
	case "image/png":
		return ".png", nil
	case "image/jpeg":
		return ".jpg", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	}
	return "", fmt.Errorf("Use SVG, PNG, JPEG, GIF or WebP")
}
func (a *App) iconAPI(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/icons/search":
		if r.Method != "GET" {
			http.Error(w, "Method", 405)
			return
		}
		source, q := r.URL.Query().Get("source"), r.URL.Query().Get("q")
		if len(q) > 100 {
			http.Error(w, "query too long", 400)
			return
		}
		type item struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		out := []item{}
		if source == "vectorlogo" {
			slug := strings.ToLower(strings.TrimSpace(q))
			if regexp.MustCompile(`^[a-z0-9-]{1,60}$`).MatchString(slug) {
				out = append(out, item{slug, "https://www.vectorlogo.zone/logos/" + slug + "/" + slug + "-icon.svg"})
			}
			reply(w, out)
			return
		}
		if source == "svgl" {
			b, e := fetchIconResource("https://api.svgl.app?search=" + url.QueryEscape(q))
			if e != nil {
				http.Error(w, e.Error(), 502)
				return
			}
			var list []struct {
				Title string          `json:"title"`
				Route json.RawMessage `json:"route"`
			}
			if e = json.Unmarshal(b, &list); e != nil {
				http.Error(w, e.Error(), 502)
				return
			}
			for _, v := range list {
				var route string
				if json.Unmarshal(v.Route, &route) != nil {
					var theme map[string]string
					_ = json.Unmarshal(v.Route, &theme)
					route = theme["dark"]
					if route == "" {
						route = theme["light"]
					}
				}
				if iconURLAllowed(route) {
					out = append(out, item{v.Title, route})
				}
				if len(out) >= 48 {
					break
				}
			}
		} else {
			prefix := ""
			if source == "simpleicons" {
				prefix = "simple-icons"
			} else if source == "devicon" {
				prefix = "devicon"
			} else if source != "iconify" {
				http.Error(w, "unknown source", 400)
				return
			}
			raw := "https://api.iconify.design/search?query=" + url.QueryEscape(q) + "&limit=48"
			if prefix != "" {
				raw += "&prefix=" + prefix
			}
			b, e := fetchIconResource(raw)
			if e != nil {
				http.Error(w, e.Error(), 502)
				return
			}
			var result struct {
				Icons []string `json:"icons"`
			}
			if e = json.Unmarshal(b, &result); e != nil {
				http.Error(w, e.Error(), 502)
				return
			}
			for _, name := range result.Icons {
				parts := strings.SplitN(name, ":", 2)
				if len(parts) == 2 {
					out = append(out, item{name, "https://api.iconify.design/" + parts[0] + "/" + parts[1] + ".svg?color=%23edf3ee"})
				}
			}
		}
		reply(w, out)
	case "/api/icons/preview":
		b, e := fetchIconResource(r.URL.Query().Get("url"))
		if e == nil {
			_, e = mediaExtension(b)
		}
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
		_, _ = w.Write(b)
	case "/api/media/import":
		if r.Method != "POST" {
			http.Error(w, "Method", 405)
			return
		}
		var q struct {
			Kind string `json:"kind"`
			URL  string `json:"url"`
			Data string `json:"data"`
		}
		if !decode(w, r, &q) {
			return
		}
		if q.Kind != "icon" && q.Kind != "image" {
			http.Error(w, "kind", 400)
			return
		}
		var b []byte
		var e error
		if q.URL != "" {
			b, e = fetchIconResource(q.URL)
		} else {
			b, e = base64.StdEncoding.DecodeString(q.Data)
		}
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		ext, e := mediaExtension(b)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		hash := sha256.Sum256(b)
		name := fmt.Sprintf("asset-%x%s", hash[:8], ext)
		path := filepath.Join(a.root, q.Kind, name)
		if e = os.WriteFile(path, b, 0600); e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		if q.URL != "" {
			meta, _ := json.MarshalIndent(map[string]string{"url": q.URL, "importedAt": time.Now().UTC().Format(time.RFC3339)}, "", "  ")
			_ = os.WriteFile(path+".source.json", meta, 0600)
		}
		reply(w, map[string]string{"path": "/media/" + q.Kind + "/" + name, "name": name})
	}
}
