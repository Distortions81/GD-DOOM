package main

import (
	"flag"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	dir := flag.String("dir", defaultServeDir(), "directory to serve")
	addr := flag.String("addr", ":8000", "listen address")
	flag.Parse()

	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		log.Fatalf("register wasm mime: %v", err)
	}

	log.Printf("serving %s on http://localhost%s", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, newHandler(*dir)))
}

func defaultServeDir() string {
	if hasAppFiles(".") {
		return "."
	}
	buildDir := filepath.Join("build", "wasm")
	if hasAppFiles(buildDir) {
		return buildDir
	}
	return "."
}

func hasAppFiles(dir string) bool {
	for _, name := range []string{"index.html", "player.html", "launch.js", "wasm_exec.js", "gddoom.wasm"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

func newHandler(dir string) http.Handler {
	files := map[string]string{
		"/":             "index.html",
		"/index.html":   "index.html",
		"/player.html":  "player.html",
		"/favicon.ico":  "",
		"/launch.js":    "launch.js",
		"/wasm_exec.js": "wasm_exec.js",
		"/gddoom.wasm":  "gddoom.wasm",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if name == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		path := filepath.Join(dir, name)
		encoding := ""
		if name == "gddoom.wasm" && acceptsGzip(r.Header.Get("Accept-Encoding")) {
			gzPath := path + ".gz"
			if _, err := os.Stat(gzPath); err == nil {
				path = gzPath
				encoding = "gzip"
			}
		}
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if encoding != "" {
			w.Header().Set("Content-Encoding", encoding)
			w.Header().Add("Vary", "Accept-Encoding")
			if ctype := mime.TypeByExtension(filepath.Ext(name)); ctype != "" {
				w.Header().Set("Content-Type", ctype)
			}
		}
		if name == "gddoom.wasm" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}

		http.ServeContent(w, r, name, info.ModTime(), f)
	})
}

func acceptsGzip(value string) bool {
	for _, part := range strings.Split(value, ",") {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		segments := strings.Split(token, ";")
		name := strings.TrimSpace(segments[0])
		if !strings.EqualFold(name, "gzip") {
			continue
		}
		q := 1.0
		for _, seg := range segments[1:] {
			seg = strings.TrimSpace(seg)
			if !strings.HasPrefix(strings.ToLower(seg), "q=") {
				continue
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(seg[2:]), 64)
			if err != nil {
				return false
			}
			q = v
			break
		}
		if q > 0 {
			return true
		}
	}
	return false
}
