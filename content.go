package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Page is one Markdown file under content/.
type Page struct {
	Section     string // "" for top-level pages such as about.md, otherwise the folder name ("post", "projects")
	Slug        string
	URL         string // site-relative, always ends in "/"
	Title       string
	Description string
	Author      string
	Date        time.Time
	Lastmod     time.Time
	Tags        []string
	Aliases     []string
	Draft       bool
	Params      map[string]any // every front matter key, lower-cased, for section-specific templates
	Content     template.HTML
	Summary     template.HTML
	Truncated   bool // Summary is shorter than Content
	ReadingTime int  // minutes
	Source      string
}

// Param returns a front matter value, so templates can read fields the
// generator does not know about, like {{ .Param "status" }} on a project.
func (p *Page) Param(key string) any { return p.Params[strings.ToLower(key)] }

// Section is a folder under content/, described by its optional _index.md.
type Section struct {
	Name        string
	URL         string
	Title       string
	Description string
	Content     template.HTML
	Pages       []*Page // newest first
}

const moreMarker = "<!--more-->"

var fmDelims = map[string]string{"+++": "toml", "---": "yaml"}

// splitFrontMatter separates a TOML (+++) or YAML (---) header from the body.
func splitFrontMatter(src []byte) (map[string]any, []byte, error) {
	meta := map[string]any{}
	src = bytes.TrimPrefix(src, []byte("\ufeff"))
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	firstLine, rest, _ := bytes.Cut(src, []byte("\n"))
	delim := string(bytes.TrimSpace(firstLine))
	format, ok := fmDelims[delim]
	if !ok {
		return meta, src, nil
	}
	end := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(delim) + `[ \t]*$`).FindIndex(rest)
	if end == nil {
		return nil, nil, fmt.Errorf("front matter opened with %q but never closed", delim)
	}
	header, body := rest[:end[0]], rest[end[1]:]
	var err error
	if format == "toml" {
		_, err = toml.Decode(string(header), &meta)
	} else {
		err = yaml.Unmarshal(header, &meta)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s front matter: %w", format, err)
	}
	lower := make(map[string]any, len(meta))
	for k, v := range meta {
		lower[strings.ToLower(k)] = v
	}
	return lower, bytes.TrimLeft(body, "\n"), nil
}

func metaString(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func metaStrings(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, s := range v {
			out = append(out, fmt.Sprint(s))
		}
		return out
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func metaBool(m map[string]any, key string) bool {
	b, _ := m[key].(bool)
	return b
}

var dateLayouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}

func metaTime(m map[string]any, key string) (time.Time, error) {
	switch v := m[key].(type) {
	case nil:
		return time.Time{}, nil
	case time.Time:
		// TOML local dates carry a placeholder location; treat them as UTC like Hugo does.
		return time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), time.UTC), nil
	case string:
		if v == "" {
			return time.Time{}, nil
		}
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("%s: cannot read %q as a date (use YYYY-MM-DD)", key, v)
	default:
		return time.Time{}, fmt.Errorf("%s: unexpected type %T", key, v)
	}
}

// loadPage reads and renders one Markdown file. section is "" for top-level pages.
func (b *builder) loadPage(file, section string) (*Page, error) {
	src, err := fs.ReadFile(b.src, file)
	if err != nil {
		return nil, err
	}
	meta, body, err := splitFrontMatter(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	p := &Page{
		Section:     section,
		Slug:        strings.TrimSuffix(path.Base(file), ".md"),
		Title:       metaString(meta, "title"),
		Description: metaString(meta, "description"),
		Author:      metaString(meta, "author"),
		Tags:        metaStrings(meta, "tags"),
		Aliases:     metaStrings(meta, "aliases"),
		Draft:       metaBool(meta, "draft"),
		Params:      meta,
		Source:      file,
	}
	if s := metaString(meta, "slug"); s != "" {
		p.Slug = s
	}
	if p.Date, err = metaTime(meta, "date"); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if p.Lastmod, err = metaTime(meta, "lastmod"); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if p.Lastmod.IsZero() {
		p.Lastmod = p.Date
	}
	if p.Author == "" {
		p.Author = b.cfg.Author
	}

	switch u := metaString(meta, "url"); {
	case u != "":
		p.URL = "/" + strings.Trim(u, "/") + "/"
	case section == "":
		p.URL = "/" + p.Slug + "/"
	default:
		p.URL = "/" + section + "/" + p.Slug + "/"
	}

	full, err := b.markdown(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	p.Content = full
	p.ReadingTime = int(math.Max(1, math.Ceil(float64(wordCount(string(body)))/200)))

	switch {
	case metaString(meta, "summary") != "":
		p.Summary, err = b.markdown([]byte(metaString(meta, "summary")))
		p.Truncated = true
	case bytes.Contains(body, []byte(moreMarker)):
		before, _, _ := bytes.Cut(body, []byte(moreMarker))
		p.Summary, err = b.markdown(before)
		p.Truncated = true
	default:
		var text string
		// Headings make poor summaries, so the automatic one uses body text only.
		text, p.Truncated = firstWords(stripTags(headingRE.ReplaceAllString(string(full), "")), 45)
		p.Summary = template.HTML(template.HTMLEscapeString(text))
	}
	return p, err
}

// loadContent walks content/ and returns top-level pages and sections.
func (b *builder) loadContent() ([]*Page, map[string]*Section, error) {
	var pages []*Page
	sections := map[string]*Section{}
	entries, err := fs.ReadDir(b.src, "content")
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir():
			s, err := b.loadSection(name)
			if err != nil {
				return nil, nil, err
			}
			sections[name] = s
		case strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, "_"):
			p, err := b.loadPage("content/"+name, "")
			if err != nil {
				return nil, nil, err
			}
			if b.include(p) {
				pages = append(pages, p)
			}
		}
	}
	return pages, sections, nil
}

func (b *builder) loadSection(name string) (*Section, error) {
	dir := "content/" + name
	s := &Section{Name: name, URL: "/" + name + "/", Title: titleCase(name)}
	if idx, err := b.loadPage(dir+"/_index.md", name); err == nil {
		s.Title, s.Description, s.Content = idx.Title, idx.Description, idx.Content
		if s.Description == "" {
			s.Description = metaString(idx.Params, "summary")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	entries, err := fs.ReadDir(b.src, dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		// Files starting with "_" are section metadata or templates to copy, never pages.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		p, err := b.loadPage(dir+"/"+e.Name(), name)
		if err != nil {
			return nil, err
		}
		if b.include(p) {
			s.Pages = append(s.Pages, p)
		}
	}
	sortPages(s.Pages)
	return s, nil
}

// include drops drafts and future-dated pages unless building with -drafts.
func (b *builder) include(p *Page) bool {
	if b.opts.Drafts {
		return true
	}
	return !p.Draft && !p.Date.After(b.now)
}

func sortPages(pages []*Page) {
	sort.SliceStable(pages, func(i, j int) bool {
		if !pages[i].Date.Equal(pages[j].Date) {
			return pages[i].Date.After(pages[j].Date)
		}
		return pages[i].Title < pages[j].Title
	})
}

var (
	tagRE     = regexp.MustCompile(`<[^>]*>`)
	headingRE = regexp.MustCompile(`(?s)<h[1-6][^>]*>.*?</h[1-6]>`)
)

func stripTags(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	for _, r := range [][2]string{{"&amp;", "&"}, {"&lt;", "<"}, {"&gt;", ">"}, {"&quot;", `"`}, {"&#39;", "'"},
		{"&rsquo;", "’"}, {"&lsquo;", "‘"}, {"&rdquo;", "”"}, {"&ldquo;", "“"},
		{"&hellip;", "…"}, {"&ndash;", "–"}, {"&mdash;", "—"}} {
		s = strings.ReplaceAll(s, r[0], r[1])
	}
	return strings.Join(strings.Fields(s), " ")
}

func firstWords(s string, n int) (string, bool) {
	words := strings.Fields(s)
	if len(words) <= n {
		return s, false
	}
	return strings.Join(words[:n], " ") + "…", true
}

func wordCount(s string) int { return len(strings.Fields(s)) }

// urlize turns a tag like "Stream of Consciousness" into "stream-of-consciousness".
func urlize(s string) string {
	var sb strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.':
			sb.WriteRune(r)
			dash = false
		case !dash && sb.Len() > 0:
			sb.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(sb.String(), "-")
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// outputPath maps a site URL like "/post/hello/" to "post/hello/index.html" inside dir.
func outputPath(dir, url string) string {
	rel := strings.TrimPrefix(url, "/")
	if rel == "" || strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	return filepath.Join(dir, filepath.FromSlash(rel))
}
