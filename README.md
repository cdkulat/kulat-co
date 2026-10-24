# kulat.co

Christian Kulat's blog and portfolio. The site is plain Markdown turned into static HTML by a small Go program in this repo (no Hugo, no theme submodule).

## Everyday use

You need Go 1.24 or newer.

```sh
go run . -serve                # preview at http://localhost:1313, rebuilds and reloads on save (drafts shown)
go run . new post my-new-post  # creates content/post/my-new-post.md as a draft
go run .                       # production build into public/ (drafts and future-dated posts skipped)
go test ./...                  # checks every URL the old Hugo site published still gets built
```

To publish a post, set `draft = false` (or delete the line) and push.

## Where things live

| Path | What it is |
| --- | --- |
| `site.toml` | Site title, home page intro, menu, social links, posts per page |
| `content/*.md` | Top-level pages: `/about/`, `/contact/`, `/resume/` |
| `content/post/*.md` | Blog posts at `/post/<file-name>/` |
| `content/<section>/` | Any other folder becomes a section with a list page at `/<section>/` |
| `content/<section>/_index.md` | Optional title and intro for a section's list page |
| `templates/` | Go `html/template` files; `base.html` wraps every page |
| `templates/<section>/single.html` | Optional per-section override, e.g. `templates/projects/single.html` |
| `static/` | Copied as-is into the site root (CSS, images, `_redirects`) |
| `archetypes/<section>.md` | Starting text for `go run . new <section> <slug>` |

Files whose name starts with `_` are never published as pages.

## Front matter

Either TOML between `+++` lines or YAML between `---` lines.

| Field | Notes |
| --- | --- |
| `title` | Page title |
| `date` | `YYYY-MM-DD`. Posts dated in the future wait until a build after that date |
| `tags` | List of strings. Each gets `/tags/<tag>/` and `/tags/<tag>/index.xml` |
| `draft` | `true` keeps it out of production builds |
| `description` | Used for the `<meta name="description">` |
| `summary` | Overrides the automatic summary on list pages (or put `<!--more-->` in the body) |
| `author`, `lastmod`, `slug`, `url`, `aliases` | Optional, same meaning as in Hugo |

Every other field is still available to templates through `.Page.Param "name"`, which is how section-specific fields like a project's `status` or `stack` reach `templates/projects/single.html`.

## What gets built

- Home page with the intro and the newest posts, 5 per page (`/page/2/`, ...)
- `/post/`, `/tags/` and `/tags/<tag>/`, also paginated
- RSS at `/index.xml`, `/post/index.xml` and `/tags/<tag>/index.xml`
- `/sitemap.xml`, `/robots.txt`, `/404.html`
- `/css/syntax.css` for code highlighting (light and dark Chroma styles set in `site.toml`)

## Deploying on Cloudflare Pages

Connect the repo in Cloudflare Pages with:

- Build command: `go run .`
- Build output directory: `public`
- Environment variable: `GO_VERSION` = `1.24`

`static/_redirects` keeps the old Hugo-only URLs (`/categories/`, `/tags/index.xml`) pointing somewhere useful.
