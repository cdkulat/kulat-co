// Command kulat-co builds the kulat.co static site.
//
//	go run .                     build into public/
//	go run . -serve              build, serve on localhost:1313, rebuild on save
//	go run . new post my-title   start a new post from archetypes/
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) > 1 && os.Args[1] == "new" {
		if err := newContent(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	var opts Options
	serve := flag.Bool("serve", false, "serve the site locally and rebuild on changes")
	addr := flag.String("addr", "localhost:1313", "address for -serve")
	flag.StringVar(&opts.Src, "src", ".", "site folder containing site.toml, content/, templates/ and static/")
	flag.StringVar(&opts.Out, "out", "public", "output folder (emptied on every build)")
	flag.BoolVar(&opts.Drafts, "drafts", false, "include drafts and future-dated pages (on by default with -serve)")
	flag.StringVar(&opts.BaseURL, "baseurl", "", "override baseURL from site.toml")
	flag.Parse()

	if *serve {
		if !flagSet("drafts") {
			opts.Drafts = true
		}
		if err := Serve(opts, *addr); err != nil {
			log.Fatal(err)
		}
		return
	}
	start := time.Now()
	if err := Build(opts); err != nil {
		log.Fatal(err)
	}
	log.Printf("built %s in %s", opts.Out, time.Since(start).Round(time.Millisecond))
}

func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

const defaultArchetype = `+++
title = "{{ .Title }}"
date = "{{ .Date }}"
author = "{{ .Author }}"
tags = []
draft = true
+++

`

// newContent creates content/<section>/<slug>.md from archetypes/<section>.md,
// falling back to archetypes/default.md and then a built-in post header.
func newContent(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: go run . new <section> <slug>   e.g. go run . new post learning-go")
	}
	section, slug := args[0], urlize(args[1])
	dst := filepath.Join("content", section, slug+".md")
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	}

	src := defaultArchetype
	for _, name := range []string{section + ".md", "default.md"} {
		if b, err := os.ReadFile(filepath.Join("archetypes", name)); err == nil {
			src = string(b)
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	t, err := template.New("archetype").Parse(src)
	if err != nil {
		return err
	}
	author := ""
	if cfg, err := loadConfig(os.DirFS(".")); err == nil {
		author = cfg.Author
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]string{
		"Title":  titleCase(strings.ReplaceAll(slug, "-", " ")),
		"Slug":   slug,
		"Date":   time.Now().Format("2006-01-02"),
		"Author": author,
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Println("created", dst)
	return nil
}
