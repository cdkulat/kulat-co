package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// Config is site.toml.
type Config struct {
	BaseURL        string   `toml:"baseURL"`
	Title          string   `toml:"title"`
	Description    string   `toml:"description"`
	Author         string   `toml:"author"`
	Language       string   `toml:"language"`
	Paginate       int      `toml:"paginate"`
	MainSections   []string `toml:"mainSections"`
	HighlightLight string   `toml:"highlightStyle"`
	HighlightDark  string   `toml:"highlightStyleDark"`
	Home           struct {
		Title string `toml:"title"`
		Intro string `toml:"intro"`
	} `toml:"home"`
	Menu   []Link `toml:"menu"`
	Social []Link `toml:"social"`
}

type Link struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

// Site is what every template sees as .Site.
type Site struct {
	*Config
	Intro    template.HTML
	Pages    []*Page             // top-level pages: about, contact, resume
	Sections map[string]*Section // every folder under content/
	Posts    []*Page             // pages from mainSections, newest first
	Tags     []*Tag              // most used first
	Dev      bool                // true under -serve; turns on live reload
	Built    time.Time
}

type Tag struct {
	Name  string
	Slug  string
	URL   string
	Pages []*Page
}

// Pager describes one page of a paginated list.
type Pager struct {
	Number, Total    int
	PrevURL, NextURL string
}

// View is the data passed to a template.
type View struct {
	Site        *Site
	Kind        string // home, page, single, list, tags, tag, 404
	Title       string
	Description string
	URL         string
	FeedURL     string
	Page        *Page
	Section     *Section
	Tag         *Tag
	Pages       []*Page
	Pager       *Pager
}

type Options struct {
	Src     string
	Out     string
	Drafts  bool
	Dev     bool
	BaseURL string // overrides site.toml, used by -serve
}

type builder struct {
	opts    Options
	src     fs.FS
	cfg     *Config
	md      goldmark.Markdown
	now     time.Time
	site    *Site
	tmpl    map[string]*template.Template
	sitemap []sitemapURL
}

func loadConfig(fsys fs.FS) (*Config, error) {
	raw, err := fs.ReadFile(fsys, "site.toml")
	if err != nil {
		return nil, err
	}
	cfg := &Config{Paginate: 5, MainSections: []string{"post"}, Language: "en",
		HighlightLight: "github", HighlightDark: "github-dark"}
	if _, err := toml.Decode(string(raw), cfg); err != nil {
		return nil, fmt.Errorf("site.toml: %w", err)
	}
	if cfg.Paginate < 1 {
		cfg.Paginate = 5
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/") + "/"
	return cfg, nil
}

// Build renders the whole site from opts.Src into opts.Out.
func Build(opts Options) error {
	b := &builder{opts: opts, src: os.DirFS(opts.Src), now: time.Now().UTC(), tmpl: map[string]*template.Template{}}
	var err error
	if b.cfg, err = loadConfig(b.src); err != nil {
		return err
	}
	if opts.BaseURL != "" {
		b.cfg.BaseURL = strings.TrimSuffix(opts.BaseURL, "/") + "/"
	}
	b.md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, extension.Typographer, extension.Footnote,
			highlighting.NewHighlighting(highlighting.WithFormatOptions(chromahtml.WithClasses(true))),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)

	site := &Site{Config: b.cfg, Dev: opts.Dev, Built: b.now}
	b.site = site
	if site.Intro, err = b.markdown([]byte(b.cfg.Home.Intro)); err != nil {
		return err
	}
	if site.Pages, site.Sections, err = b.loadContent(); err != nil {
		return err
	}
	for _, name := range b.cfg.MainSections {
		if s := site.Sections[name]; s != nil {
			site.Posts = append(site.Posts, s.Pages...)
		}
	}
	sortPages(site.Posts)
	site.Tags = collectTags(site)

	if err := b.resetOut(); err != nil {
		return err
	}
	steps := []func() error{b.copyStatic, b.writeSyntaxCSS, b.renderAll, b.writeSitemap, b.writeRobots}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) markdown(src []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := b.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func collectTags(site *Site) []*Tag {
	bySlug := map[string]*Tag{}
	add := func(p *Page) {
		for _, name := range p.Tags {
			slug := urlize(name)
			if slug == "" {
				continue
			}
			t := bySlug[slug]
			if t == nil {
				t = &Tag{Name: name, Slug: slug, URL: "/tags/" + slug + "/"}
				bySlug[slug] = t
			}
			t.Pages = append(t.Pages, p)
		}
	}
	for _, p := range site.Pages {
		add(p)
	}
	for _, s := range site.Sections {
		for _, p := range s.Pages {
			add(p)
		}
	}
	tags := make([]*Tag, 0, len(bySlug))
	for _, t := range bySlug {
		sortPages(t.Pages)
		tags = append(tags, t)
	}
	sort.Slice(tags, func(i, j int) bool {
		if len(tags[i].Pages) != len(tags[j].Pages) {
			return len(tags[i].Pages) > len(tags[j].Pages)
		}
		return tags[i].Slug < tags[j].Slug
	})
	return tags
}

// resetOut empties the output folder, refusing anything that looks like the source tree.
func (b *builder) resetOut() error {
	out, err := filepath.Abs(b.opts.Out)
	if err != nil {
		return err
	}
	src, err := filepath.Abs(b.opts.Src)
	if err != nil {
		return err
	}
	if out == src || strings.HasPrefix(src, out+string(filepath.Separator)) {
		return fmt.Errorf("refusing to use %s as the output folder: it contains the source", out)
	}
	for _, keep := range []string{"content", "templates", "static"} {
		if out == filepath.Join(src, keep) {
			return fmt.Errorf("refusing to use %s as the output folder", out)
		}
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	return os.MkdirAll(out, 0o755)
}

func (b *builder) write(url string, data []byte) error {
	dst := outputPath(b.opts.Out, url)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (b *builder) copyStatic() error {
	if _, err := fs.Stat(b.src, "static"); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fs.WalkDir(b.src, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(b.src, p)
		if err != nil {
			return err
		}
		return b.write(strings.TrimPrefix(p, "static"), data)
	})
}

// writeSyntaxCSS emits /css/syntax.css for highlighted code blocks, with a dark variant.
func (b *builder) writeSyntaxCSS() error {
	f := chromahtml.New(chromahtml.WithClasses(true))
	var buf bytes.Buffer
	if err := f.WriteCSS(&buf, styles.Get(b.cfg.HighlightLight)); err != nil {
		return err
	}
	buf.WriteString("@media (prefers-color-scheme: dark) {\n")
	if err := f.WriteCSS(&buf, styles.Get(b.cfg.HighlightDark)); err != nil {
		return err
	}
	buf.WriteString("}\n")
	return b.write("/css/syntax.css", buf.Bytes())
}

var templateFuncs = template.FuncMap{
	"date":    func(t time.Time) string { return t.Format("January 2, 2006") },
	"isoDate": func(t time.Time) string { return t.Format("2006-01-02") },
	"urlize":  urlize,
	"lower":   strings.ToLower,
	"join":    strings.Join,
}

// template returns templates/<section>/<kind>.html if it exists, else templates/<kind>.html,
// each combined with templates/base.html and templates/partials/*.html.
func (b *builder) template(kind, section string) (*template.Template, error) {
	candidates := []string{"templates/" + kind + ".html"}
	if section != "" {
		candidates = append([]string{"templates/" + section + "/" + kind + ".html"}, candidates...)
	}
	for _, name := range candidates {
		if t, ok := b.tmpl[name]; ok {
			return t, nil
		}
		if _, err := fs.Stat(b.src, name); err != nil {
			continue
		}
		absURL := func(u string) string { return b.absURL(u) }
		t := template.New("base").Funcs(templateFuncs).Funcs(template.FuncMap{"absURL": absURL})
		files := []string{"templates/base.html"}
		if partials, _ := fs.Glob(b.src, "templates/partials/*.html"); len(partials) > 0 {
			files = append(files, partials...)
		}
		files = append(files, name)
		t, err := t.ParseFS(b.src, files...)
		if err != nil {
			return nil, err
		}
		b.tmpl[name] = t
		return t, nil
	}
	return nil, fmt.Errorf("no template for %q (looked for %s)", kind, strings.Join(candidates, ", "))
}

func (b *builder) absURL(u string) string {
	if strings.Contains(u, "://") {
		return u
	}
	return b.cfg.BaseURL + strings.TrimPrefix(u, "/")
}

func (b *builder) render(v *View, section string) error {
	v.Site = b.site
	t, err := b.template(v.Kind, section)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", v); err != nil {
		return fmt.Errorf("rendering %s: %w", v.URL, err)
	}
	return b.write(v.URL, buf.Bytes())
}

// paginate renders base, base/page/2/, ... with cfg.Paginate pages each,
// plus base/page/1/ as a redirect to base, which is what Hugo generated.
func (b *builder) paginate(v View, section string, pages []*Page) error {
	base := v.URL
	per := b.cfg.Paginate
	total := max(1, (len(pages)+per-1)/per)
	pageURL := func(n int) string {
		if n == 1 {
			return base
		}
		return fmt.Sprintf("%spage/%d/", base, n)
	}
	for n := 1; n <= total; n++ {
		pv := v
		pv.URL = pageURL(n)
		pv.Pages = pages[(n-1)*per : min(n*per, len(pages))]
		pv.Pager = &Pager{Number: n, Total: total}
		if n > 1 {
			pv.Pager.PrevURL = pageURL(n - 1)
		}
		if n < total {
			pv.Pager.NextURL = pageURL(n + 1)
		}
		if err := b.render(&pv, section); err != nil {
			return err
		}
	}
	return b.alias(base+"page/1/", base)
}

const aliasHTML = `<!doctype html><html lang="en"><head><title>%[1]s</title><link rel="canonical" href="%[1]s"><meta name="robots" content="noindex"><meta charset="utf-8"><meta http-equiv="refresh" content="0; url=%[1]s"></head></html>`

func (b *builder) alias(from, to string) error {
	return b.write(from, []byte(fmt.Sprintf(aliasHTML, template.HTMLEscapeString(b.absURL(to)))))
}

func (b *builder) renderAll() error {
	site := b.site
	lastmod := func(pages []*Page) time.Time {
		var t time.Time
		for _, p := range pages {
			if p.Lastmod.After(t) {
				t = p.Lastmod
			}
		}
		return t
	}

	// Home: intro plus the newest posts.
	home := View{Kind: "home", URL: "/", Description: site.Description, FeedURL: "/index.xml"}
	if err := b.paginate(home, "", site.Posts); err != nil {
		return err
	}
	b.addSitemap("/", lastmod(site.Posts))
	if err := b.feed("/index.xml", site.Title, site.Description, "/", site.Posts); err != nil {
		return err
	}

	for _, p := range site.Pages {
		if err := b.renderPage(p, "page"); err != nil {
			return err
		}
	}

	names := make([]string, 0, len(site.Sections))
	for name := range site.Sections {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := site.Sections[name]
		list := View{Kind: "list", URL: s.URL, Title: s.Title, Description: s.Description, Section: s, FeedURL: s.URL + "index.xml"}
		if err := b.paginate(list, name, s.Pages); err != nil {
			return err
		}
		b.addSitemap(s.URL, lastmod(s.Pages))
		if err := b.feed(s.URL+"index.xml", s.Title+" on "+site.Title, s.Description, s.URL, s.Pages); err != nil {
			return err
		}
		for _, p := range s.Pages {
			if err := b.renderPage(p, "single"); err != nil {
				return err
			}
		}
	}

	if err := b.render(&View{Kind: "tags", URL: "/tags/", Title: "Tags"}, ""); err != nil {
		return err
	}
	b.addSitemap("/tags/", time.Time{})
	for _, t := range site.Tags {
		v := View{Kind: "tag", URL: t.URL, Title: t.Name, Tag: t, FeedURL: t.URL + "index.xml"}
		if err := b.paginate(v, "", t.Pages); err != nil {
			return err
		}
		b.addSitemap(t.URL, lastmod(t.Pages))
		if err := b.feed(t.URL+"index.xml", t.Name+" on "+site.Title, "", t.URL, t.Pages); err != nil {
			return err
		}
	}

	return b.render(&View{Kind: "404", URL: "/404.html", Title: "Page not found"}, "")
}

func (b *builder) renderPage(p *Page, kind string) error {
	v := &View{Kind: kind, URL: p.URL, Title: p.Title, Description: p.Description, Page: p}
	if v.Description == "" {
		v.Description = strings.TrimSpace(stripTags(string(p.Summary)))
	}
	if err := b.render(v, p.Section); err != nil {
		return err
	}
	b.addSitemap(p.URL, p.Lastmod)
	for _, a := range p.Aliases {
		if err := b.alias("/"+strings.Trim(a, "/")+"/", p.URL); err != nil {
			return err
		}
	}
	return nil
}

// RSS 2.0, matching the shape of the feeds Hugo produced so subscribers keep working.
type rssFeed struct {
	XMLName  xml.Name   `xml:"rss"`
	Version  string     `xml:"version,attr"`
	AtomNS   string     `xml:"xmlns:atom,attr"`
	Contents string     `xml:"xmlns:content,attr"`
	Channel  rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language,omitempty"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Self          atomLink  `xml:"atom:link"`
	Items         []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PubDate     string   `xml:"pubDate"`
	GUID        string   `xml:"guid"`
	Categories  []string `xml:"category"`
	Description string   `xml:"description"`
	Content     string   `xml:"content:encoded"`
}

func (b *builder) feed(url, title, description, link string, pages []*Page) error {
	if description == "" {
		description = "Recent content on " + title
	}
	ch := rssChannel{Title: title, Link: b.absURL(link), Description: description, Language: b.cfg.Language,
		Self: atomLink{Href: b.absURL(url), Rel: "self", Type: "application/rss+xml"}}
	for i, p := range pages {
		if i == 0 {
			ch.LastBuildDate = p.Date.Format(time.RFC1123Z)
		}
		ch.Items = append(ch.Items, rssItem{
			Title: p.Title, Link: b.absURL(p.URL), GUID: b.absURL(p.URL),
			PubDate:     p.Date.Format(time.RFC1123Z),
			Categories:  p.Tags,
			Description: string(p.Summary), Content: string(p.Content),
		})
	}
	out, err := xml.MarshalIndent(rssFeed{Version: "2.0", AtomNS: "http://www.w3.org/2005/Atom",
		Contents: "http://purl.org/rss/1.0/modules/content/", Channel: ch}, "", "  ")
	if err != nil {
		return err
	}
	return b.write(url, append([]byte(xml.Header), out...))
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod,omitempty"`
}

func (b *builder) addSitemap(url string, lastmod time.Time) {
	u := sitemapURL{Loc: b.absURL(url)}
	if !lastmod.IsZero() {
		u.Lastmod = lastmod.Format("2006-01-02")
	}
	b.sitemap = append(b.sitemap, u)
}

func (b *builder) writeSitemap() error {
	sort.Slice(b.sitemap, func(i, j int) bool { return b.sitemap[i].Loc < b.sitemap[j].Loc })
	out, err := xml.MarshalIndent(struct {
		XMLName xml.Name     `xml:"urlset"`
		NS      string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: b.sitemap}, "", "  ")
	if err != nil {
		return err
	}
	return b.write("/sitemap.xml", append([]byte(xml.Header), out...))
}

func (b *builder) writeRobots() error {
	// A robots.txt in static/ wins.
	if _, err := fs.Stat(b.src, path.Join("static", "robots.txt")); err == nil {
		return nil
	}
	return b.write("/robots.txt", []byte("User-agent: *\nDisallow:\nSitemap: "+b.absURL("/sitemap.xml")+"\n"))
}
