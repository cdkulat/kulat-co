package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Serve builds the site, serves it on addr, and rebuilds whenever a source file changes.
// Pages poll /__reload and refresh themselves after each successful build.
func Serve(opts Options, addr string) error {
	opts.Dev = true
	if opts.BaseURL == "" {
		opts.BaseURL = "http://" + addr + "/"
	}

	var mu sync.RWMutex
	version := ""
	rebuild := func() {
		start := time.Now()
		mu.Lock()
		defer mu.Unlock()
		if err := Build(opts); err != nil {
			log.Printf("build failed: %v", err)
			return
		}
		version = fmt.Sprint(time.Now().UnixNano())
		log.Printf("rebuilt in %s", time.Since(start).Round(time.Millisecond))
	}
	rebuild()

	go func() {
		last := sourceStamp(opts.Src, opts.Out)
		for range time.Tick(400 * time.Millisecond) {
			if s := sourceStamp(opts.Src, opts.Out); s != last {
				last = s
				rebuild()
			}
		}
	}()

	files := http.FileServer(http.Dir(opts.Out))
	mux := http.NewServeMux()
	mux.HandleFunc("/__reload", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, version)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Cache-Control", "no-store")
		p := filepath.Join(opts.Out, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
		if _, err := os.Stat(p); err != nil {
			// Same behaviour as Cloudflare Pages: unknown paths get 404.html.
			w.WriteHeader(http.StatusNotFound)
			if b, err := os.ReadFile(filepath.Join(opts.Out, "404.html")); err == nil {
				w.Write(b)
			}
			return
		}
		files.ServeHTTP(w, r)
	})

	log.Printf("serving %s at http://%s/ (drafts: %v), Ctrl+C to stop", opts.Out, addr, opts.Drafts)
	return http.ListenAndServe(addr, mux)
}

// sourceStamp summarises modification times of everything the build reads, skipping the output folder.
func sourceStamp(src, out string) string {
	outAbs, _ := filepath.Abs(out)
	var newest time.Time
	count := 0
	for _, dir := range []string{"content", "templates", "static", "site.toml"} {
		filepath.WalkDir(filepath.Join(src, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if abs, _ := filepath.Abs(p); abs == outAbs {
				return filepath.SkipDir
			}
			if info, err := d.Info(); err == nil {
				count++
				if info.ModTime().After(newest) {
					newest = info.ModTime()
				}
			}
			return nil
		})
	}
	return fmt.Sprint(newest.UnixNano(), count)
}
