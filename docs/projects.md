# Projects content type (draft)

Each project is one Markdown file in `content/projects/`. The front matter is plain YAML,
parsed by the custom Go generator (YAML front matter + goldmark for the body). Nothing in it is Hugo-specific.

## Front matter

| Field      | Required | Type            | Notes |
|------------|----------|-----------------|-------|
| `title`    | yes      | string          | Project name as shown on the card. |
| `summary`  | yes      | string          | One or two sentences for the list page. |
| `status`   | yes      | enum            | `idea`, `in-progress`, `shipped`, `archived`. |
| `stack`    | yes      | list of strings | Languages, frameworks, infra. Rendered as chips. |
| `repo`     | no       | URL             | GitHub link. Leave empty for private or work projects. |
| `demo`     | no       | URL             | Live site or video, if there is one. |
| `date`     | yes      | YYYY-MM-DD      | When the project started. Used for sort order. |
| `ended`    | no       | YYYY-MM-DD      | Leave empty while it is ongoing. |
| `featured` | no       | bool            | Pin to the top of /projects and the home page. |
| `posts`    | no       | list of paths   | Blog posts that talk about this project, linked at the bottom. |
| `draft`    | no       | bool            | Skipped by the build when true. |

Copy `content/projects/_template.md` (it is `draft: true`) to start a new project. The generator should skip files whose name starts with `_`.

The body below the front matter is the write-up: what it does, why, how it is built, what was learned.

Gaps still to fill are marked `confirm:` as YAML comments in the front matter or HTML comments in the body, so they never show on the page.

## Where the entries came from

The two posts tagged `projects` (`hello.md`, `jobs-projects.md`) are reflections rather than
project write-ups. Only `hello.md` describes a concrete project (the goal tracker). The other
entries come from the projects already listed on `resume.md` and `about.md`, so the Projects
page matches what the resume claims.

## Suggested Go struct

```go
type Project struct {
    Title    string    `yaml:"title"`
    Summary  string    `yaml:"summary"`
    Status   string    `yaml:"status"` // idea | in-progress | shipped | archived
    Stack    []string  `yaml:"stack"`
    Repo     string    `yaml:"repo"`
    Demo     string    `yaml:"demo"`
    Date     time.Time `yaml:"date"`
    Ended    string    `yaml:"ended"`
    Featured bool      `yaml:"featured"`
    Posts    []string  `yaml:"posts"`
    Draft    bool      `yaml:"draft"`
    Body     template.HTML `yaml:"-"`
}
```

List page order: featured first, then by `date`, newest first.
