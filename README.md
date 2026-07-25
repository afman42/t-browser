# Terminal Browser (t-browser)

A lightweight **terminal-based web browser** — no mouse, no separate window, just your keyboard.  
Browse the web from anywhere you can open a terminal.

---

## ✨ Features

| Category            | What it does |
|---------------------|-------------|
| **Browse**          | Render web pages to plain text with smart content extraction (readability) |
| **Navigate**        | `j`/`k` scroll, `b`/`f` history, `l`/`i` list links/images, `/` search |
| **Images**          | Preview JPG, PNG, GIF, BMP, WebP as ASCII art in the terminal |
| **Sessions**        | Save/restore history and tabs across restarts |
| **Cookies**         | RFC 6265 compliant, SameSite enforcement, auto-save |
| **Security**        | HTML sanitisation, external resource blocking, HSTS, certificate pinning |
| **Proxy**           | Via config file or environment variable |
| **Themes**          | Dark / light with one-key toggle in settings |
| **Clipboard**       | Press `p` in the URL bar to paste from system clipboard |

---

## 🚀 Quick Start

### Requirements
- **Go 1.19+**
- `gcc` (needed for `make test-race` on Linux)

### Install & Run

```bash
# Clone and build
git clone https://github.com/yourusername/t-browser.git
cd t-browser
make build

# Open a website
./t-browser https://example.com
```

### Makefile Reference

```bash
make build          # Compile the binary
make test           # Run all tests (verbose)
make test-race      # Run tests with the race detector
make coverage       # Run tests + print per-function coverage
make coverage-html  # Open a browser with the HTML coverage report
make lint           # Run go vet (static analysis)
make run URL=…     # Build & launch the browser
make tidy           # Run go mod tidy
make clean          # Remove build artefacts
```

---

## ⌨️ Keyboard Shortcuts

| Key             | Action                          |
|-----------------|---------------------------------|
| `j` / `k`       | Scroll down / up (10 lines)    |
| `b` / `f`       | Go back / forward in history   |
| `l`             | List all links on the page      |
| `i`             | List all images on the page     |
| `/`             | Real-time search with highlights |
| `s`             | Open settings (two-column UI)  |
| `?`             | Show help overlay               |
| `Tab`           | Switch between content & URL bar |
| `p`             | Paste URL from clipboard       |
| `q` / `Ctrl+C`  | Quit                           |

### Settings Page

| Key                 | Action                                   |
|---------------------|------------------------------------------|
| `Tab`               | Switch between categories / settings     |
| `↑` / `↓` (left)    | Select a category                        |
| `Enter` (category)  | Open the settings form on the right      |
| `q` / `Esc`         | Close settings                           |
| Save button         | Persist to `config.yaml`                 |

---

## 🏗️ Architecture

### File Map

```
t-browser/
│
├── main.go                  # Entry point — creates a Browser and calls Run()
│
├── browser.go               # NewBrowser(), colour helpers, cookie export
├── types.go                 # Browser struct, Link struct, constants
├── config.go                # Config struct, Viper initialisation, config I/O
├── theme.go                 # Dark/light theme definitions and application
│
├── session.go               # Session persistence, GoBack / GoForward
├── navigation.go            # NavigateTo(), URL validation, loading indicator
├── renderer.go              # Page rendering (readability, goquery parsing)
├── html_renderer.go         # renderNode() — walks the HTML DOM tree
├── links.go                 # extractVisibleLinks() — filters links by content
├── image.go                 # Image utilities (extension check, content-type)
├── image_preview.go         # Terminal image preview modal
│
├── search.go                # Search UI (input field, result list)
├── search_highlight.go      # Regex matching and text highlighting
│
├── http.go                  # HTTPClient, fetchPageWithRedirectLimit()
├── cookie.go                # Cookie struct, domain/path matching, persistence
│
├── content_security.go      # sanitizeHTML(), blockExternalResources()
├── security.go              # Certificate pinning, HSTS store, HSTS transport
│
├── ui.go                    # UI creation (createUI()), key event dispatch
├── settings.go              # Settings data model and update logic
├── settings_modal.go        # Two-column settings modal UI
├── pagination.go            # Paginated link / image list modals
│
├── Makefile                 # Build, test, coverage, lint
└── *_test.go                # 40+ unit & integration tests alongside each module
```

### How a Page Loads

```
User types URL ----> ui.go (key handler)
                         │
                         ▼
                    navigation.go (NavigateTo)
                         │
                         ├── validateAndSanitizeURL() — blocks dangerous schemes
                         │
                         ▼
                    http.go (FetchPage)
                         │
                         ├── cookies sent with request
                         ├── cookies stored from response (with SameSite)
                         ├── HSTS header processed (Strict-Transport-Security)
                         └── binary detection + charset conversion
                         │
                         ▼
                    renderer.go (renderPage)
                         ├── content_security.go: sanitizeHTML() strips scripts/iframes
                         ├── goquery: extract links & images
                         ├── content_security.go: blockExternalResources()
                         ├── go-readability: extract main article content
                         └── html_renderer.go: renderNode() for fallback path
                         │
                         ▼
                    textView.SetText() — content displayed in terminal
```

### Module Groups

| Group                | Files |
|----------------------|-------|
| **Core**             | `main.go`, `browser.go`, `types.go`, `config.go`, `theme.go` |
| **Navigation**       | `navigation.go`, `session.go` |
| **HTTP & Cookies**   | `http.go`, `cookie.go` |
| **Rendering**        | `renderer.go`, `html_renderer.go`, `links.go`, `image.go`, `image_preview.go` |
| **Search**           | `search.go`, `search_highlight.go` |
| **Security**         | `content_security.go`, `security.go` |
| **UI**               | `ui.go`, `settings.go`, `settings_modal.go`, `pagination.go` |

---

## ⚙️ Configuration

Configuration is **auto-created** on first launch. Press `s` to open the settings UI.

| Platform | File |
|----------|------|
| Linux    | `~/.config/t-browser/config.yaml` |
| macOS    | `~/Library/Application Support/t-browser/config.yaml` |
| Windows  | `%APPDATA%\t-browser\config.yaml` |

### All Settings

| Setting                     | Type     | Default                     | Category    | Description |
|-----------------------------|----------|-----------------------------|-------------|-------------|
| `user_agent`                | string   | Chrome 91 UA                | Browser     | HTTP User-Agent header |
| `request_timeout`           | int      | `30` seconds                | Browser     | Request timeout |
| `proxy`                     | string   | —                           | Network     | Proxy URL |
| `max_redirects`             | int      | `10`                        | Network     | Max HTTP redirects |
| `enable_pinning`            | bool     | `false`                     | Network     | Certificate public-key pinning |
| `enable_hsts`               | bool     | `true`                      | Network     | HSTS (Strict-Transport-Security) |
| `max_page_size`             | int      | `50` MB                     | Content     | Max response body |
| `max_image_size`            | int      | `5` MB                      | Content     | Max image file size |
| `enable_images`             | bool     | `true`                      | Content     | Load images from pages |
| `enable_content_security`   | bool     | `true`                      | Content     | Strip scripts, iframes, event handlers |
| `block_external_resources`  | bool     | `true`                      | Content     | Block cross-origin resources |
| `theme`                     | string   | `dark`                      | UI          | Colour theme |
| `show_images`               | bool     | `true`                      | UI          | Display images as ASCII |
| `word_wrap`                 | bool     | `true`                      | UI          | Soft-wrap long lines |
| `enable_cookies`            | bool     | `true`                      | Privacy     | Cookie storage |
| `cookie_auto_save`          | bool     | `true`                      | Privacy     | Persist cookies to disk |
| `enforce_same_site`         | bool     | `true`                      | Privacy     | SameSite=Strict enforcement |
| `session_auto_save`         | bool     | `true`                      | Privacy     | Auto-save sessions |

### Proxy (Alternative)

```bash
export PROXY=http://your-proxy:port
./t-browser
```

---

## 🛡️ Security

### Always-Enabled Defences

| Defence                   | What it blocks |
|---------------------------|----------------|
| **URL scheme filter**     | `javascript:`, `data:`, `file:`, `vbscript:` |
| **Internal address block**| `localhost`, `127.*`, `10.*`, `192.168.*`, `172.16-31.*` |
| **Open redirect guard**   | Redirects must stay on the same host or a subdomain |
| **Content-type gate**     | Non-HTML responses (images, JSON, etc.) are rejected |
| **Binary magic check**    | PNG, JPEG, GIF, ZIP, PDF signatures detected and rejected |
| **Size limits**           | 50 MB pages, 5 MB images |
| **Charset conversion**    | Latin-1, ISO-8859-*, UTF-16 → UTF-8 |
| **tview injection**       | Brackets escaped to prevent format-code injection |
| **Whitespace normaliser** | Consecutive blank lines collapsed |

### Configurable Defences

| Feature                   | Enabled by default? | Config key | How it works |
|---------------------------|:---:|------------|--------------|
| **HTML sanitisation**     | ✅ | `enable_content_security` | Strips `<script>`, `<iframe>`, `<object>`, `<embed>`, `<applet>`, all `on*` event handlers, and `javascript:`/`vbscript:` URLs from HTML before rendering |
| **External resource blocking** | ✅ | `block_external_resources` | Images, iframes, scripts, stylesheets, and `<source>` elements from other origins have their `src`/`href` removed before the page is rendered |
| **SameSite enforcement**  | ✅ | `enforce_same_site` | Cookies with `SameSite=Strict` are not sent on cross-domain requests. `SameSite=Lax` always allowed (all requests are navigations). `SameSite=None` always allowed. |
| **HSTS**                  | ✅ | `enable_hsts` | `Strict-Transport-Security` headers are parsed and cached per domain. HTTP requests to HSTS hosts are transparently upgraded to HTTPS. Policies persisted to `~/.config/t-browser/hsts/policies.json`. |
| **Certificate pinning**   | ❌ | `enable_pinning` + `pinned_keys` | TLS `VerifyConnection` callback checks SHA-256 hashes of the server's SubjectPublicKeyInfo against a configurable list of base64-encoded fingerprints. Standard X.509 verification still runs. |

### Setting Pinned Keys

```yaml
# ~/.config/t-browser/config.yaml
enable_pinning: true
pinned_keys:
  - "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="   # Replace with your key
  - "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="   # Backup key
```

Keys are 32-byte SHA-256 hashes of DER-encoded SubjectPublicKeyInfo, base64-encoded.

---

## 📝 Notes

- **No JavaScript engine** — SPAs and JS-heavy sites won't render as expected
- **Text-first display** — simplifies complex layouts into readable paragraphs
- **Fast & lightweight** — designed for minimal CPU/memory usage
- **Keyboard-only** — fully accessible from any terminal

---

## 🤝 Contributing

```bash
make test           # Run all tests
make test-race      # Run with race detector
make lint           # Run go vet
make coverage       # Check per-function coverage
```

All source files follow Go conventions. Tests live alongside the module they test (`*_test.go`).
