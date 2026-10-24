package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The URLs Hugo published. Changing any of these breaks inbound links or feed subscriptions.
var hugoURLs = []string{
	"index.html", "index.xml", "sitemap.xml", "robots.txt", "404.html", "page/1/index.html",
	"about/index.html", "contact/index.html", "resume/index.html",
	"post/index.html", "post/index.xml", "post/page/1/index.html",
	"post/first/index.html", "post/hello/index.html", "post/jobs-projects/index.html",
	"tags/index.html",
	"tags/engineering/index.html", "tags/engineering/index.xml", "tags/engineering/page/1/index.html",
	"tags/projects/index.html", "tags/projects/index.xml", "tags/projects/page/1/index.html",
	"tags/stream-of-consciousness/index.html", "tags/stream-of-consciousness/index.xml",
	"tags/stream-of-consciousness/page/1/index.html",
	"_redirects", // covers /categories/ and /tags/index.xml
}

func TestBuildKeepsHugoURLs(t *testing.T) {
	out := t.TempDir()
	if err := Build(Options{Src: ".", Out: out}); err != nil {
		t.Fatal(err)
	}
	for _, u := range hugoURLs {
		if _, err := os.Stat(filepath.Join(out, u)); err != nil {
			t.Errorf("missing /%s", u)
		}
	}
	sitemap := read(t, out, "sitemap.xml")
	for _, loc := range []string{"https://kulat.co/post/hello/", "https://kulat.co/tags/projects/", "https://kulat.co/resume/"} {
		if !strings.Contains(sitemap, "<loc>"+loc+"</loc>") {
			t.Errorf("sitemap missing %s", loc)
		}
	}
}

// newSite copies the real site.toml and templates into a temp folder with the given content files.
func newSite(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "templates"), os.DirFS("templates")); err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("site.toml")
	if err != nil {
		t.Fatal(err)
	}
	files["site.toml"] = string(cfg)
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func build(t *testing.T, src string, opts Options) string {
	t.Helper()
	opts.Src, opts.Out = src, filepath.Join(t.TempDir(), "public")
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	return opts.Out
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPaginationFivePerPage(t *testing.T) {
	files := map[string]string{}
	for i := 1; i <= 12; i++ {
		files[fmt.Sprintf("content/post/p%02d.md", i)] = fmt.Sprintf("+++\ntitle = \"Post %02d\"\ndate = \"2024-01-%02d\"\ntags = [\"Go Stuff\"]\n+++\nbody\n", i, i)
	}
	out := build(t, newSite(t, files), Options{})
	for _, f := range []string{"page/2/index.html", "page/3/index.html", "post/page/3/index.html", "tags/go-stuff/page/3/index.html"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "page/4/index.html")); err == nil {
		t.Error("unexpected page 4")
	}
	home := read(t, out, "index.html")
	if strings.Count(home, `class="entry"`) != 5 || !strings.Contains(home, "Post 12") || strings.Contains(home, "Post 07") {
		t.Error("home page 1 should hold the 5 newest posts")
	}
	if !strings.Contains(read(t, out, "page/3/index.html"), "Post 01") {
		t.Error("oldest post should be on the last page")
	}
}

func TestDraftsAndFuturePostsSkipped(t *testing.T) {
	src := newSite(t, map[string]string{
		"content/post/live.md":   "+++\ntitle = \"Live\"\ndate = \"2024-01-01\"\n+++\n",
		"content/post/draft.md":  "+++\ntitle = \"Draft\"\ndate = \"2024-01-01\"\ndraft = true\n+++\n",
		"content/post/future.md": "+++\ntitle = \"Future\"\ndate = \"2999-01-01\"\n+++\n",
	})
	out := build(t, src, Options{})
	for _, f := range []string{"post/draft/index.html", "post/future/index.html"} {
		if _, err := os.Stat(filepath.Join(out, f)); err == nil {
			t.Errorf("%s should not be published", f)
		}
	}
	out = build(t, src, Options{Drafts: true})
	if _, err := os.Stat(filepath.Join(out, "post/draft/index.html")); err != nil {
		t.Error("-drafts should include drafts")
	}
}

func TestCodeHighlighting(t *testing.T) {
	out := build(t, newSite(t, map[string]string{
		"content/post/code.md": "+++\ntitle = \"Code\"\ndate = \"2024-01-01\"\n+++\n```go\nfunc main() {}\n```\n",
	}), Options{})
	if !strings.Contains(read(t, out, "post/code/index.html"), `class="chroma"`) {
		t.Error("code block was not highlighted")
	}
	if !strings.Contains(read(t, out, "css/syntax.css"), ".chroma") {
		t.Error("syntax.css missing chroma rules")
	}
}

// Projects use YAML front matter with their own fields; a section template can read them via .Page.Param.
func TestYAMLSectionWithCustomTemplate(t *testing.T) {
	src := newSite(t, map[string]string{
		"content/projects/_index.md":     "---\ntitle: \"Projects\"\nsummary: \"Things I've built.\"\n---\n",
		"content/projects/_template.md":  "---\ntitle: \"Template\"\n---\n",
		"content/projects/tracker.md":    "---\ntitle: \"Goal tracker\"\nstatus: \"in-progress\"\nstack: [\"Go\", \"REST API\"]\ndate: 2024-11-10\nfeatured: true\n---\n## What it does\n",
		"templates/projects/single.html": `{{define "main"}}<p class="status">{{.Page.Param "status"}}</p><p>{{range .Page.Param "stack"}}[{{.}}]{{end}}</p>{{end}}`,
	})
	out := build(t, src, Options{})
	page := read(t, out, "projects/tracker/index.html")
	if !strings.Contains(page, `<p class="status">in-progress</p>`) || !strings.Contains(page, "[Go][REST API]") {
		t.Errorf("project template did not see front matter:\n%s", page)
	}
	if !strings.Contains(read(t, out, "projects/index.html"), "<h1>Projects</h1>") {
		t.Error("section title from YAML _index.md not used")
	}
	if _, err := os.Stat(filepath.Join(out, "projects/_template/index.html")); err == nil {
		t.Error("files starting with _ must be skipped")
	}
	if strings.Contains(read(t, out, "index.xml"), "Goal tracker") {
		t.Error("projects are not a main section and should stay out of the main feed")
	}
}

func TestFrontMatterDates(t *testing.T) {
	for name, src := range map[string]string{
		"toml string": "+++\ndate = \"2024-11-10\"\n+++\n",
		"toml date":   "+++\ndate = 2024-11-10\n+++\n",
		"yaml":        "---\ndate: 2024-11-10\n---\n",
		"yaml quoted": "---\ndate: \"2024-11-10\"\n---\n",
	} {
		meta, _, err := splitFrontMatter([]byte(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		d, err := metaTime(meta, "date")
		if err != nil || d.Format("2006-01-02") != "2024-11-10" {
			t.Errorf("%s: got %v, %v", name, d, err)
		}
	}
}
