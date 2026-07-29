# Terminal Browser (t-browser)

A lightweight **terminal-based web browser** — no mouse, no separate window, just your keyboard.  
Browse the web from anywhere you can open a terminal.

---

## ✨ Features

| Category            | What it does |
|---------------------|-------------|
| **Browse**          | Render web pages to plain text with smart content extraction (readability) |
| **Navigate**        | `j`/`k` scroll, `b`/`f` history, `Ctrl+L`/`i` list links/images, `/` search, `n`/`N` next/prev match |
| **Tabs**            | `Ctrl+T` new tab, `Ctrl+W` close tab, `>` next, `<` prev |
| **Images**          | Preview JPG, PNG, GIF, BMP, WebP as ASCII art in the terminal |
| **Sessions**        | Save/restore history and tabs across restarts |
| **Cookies**         | RFC 6265 compliant, SameSite enforcement, auto-save |
| **Security**        | HTML sanitisation, external resource blocking, HSTS, certificate pinning, sanitisation logging |
| **Proxy**           | Via config file or environment variable |
| **Themes**          | Dark / light with one-key toggle in settings |
| **Clipboard**       | Press `p` in the URL bar to paste from system clipboard |
| **Web Search**      | Type a search query in the URL bar — it goes to your search engine |
| **HTTP/2**          | Automatic HTTP/2 with HTTP/1.1 fallback |
| **Compression**     | gzip, deflate, and Brotli decompression |
| **HTTP Caching**    | ETag / Last-Modified conditional requests (304 Not Modified) + optional client-side TTL |
| **Retry**           | Automatic retry of 429 / 502 / 503 / 504 with exponential backoff and `Retry-After` support |
| **Cancel Loading**  | Press `Esc` to abort a slow page load |
| **Table Rendering** | HTML tables rendered as aligned text grids with borders |
| **Tab Spinner**     | Loading tabs show an animated braille spinner in the tab bar |
| **Status Toasts**   | Transient notifications in the status bar (e.g. tracking params stripped) |
| **Link/Image Filter** | Type-to-filter inside the links/images modals |
| **Tracking-Param Stripping** | Removes `utm_*`, `fbclid`, `gclid`, etc. from navigated URLs |
| **Domain Blocklist**  | Blocklisted domains (with subdomain matching) are rejected at navigation |

---

## 🚀 Quick Start

### Requirements
- **Go 1.24+**
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
| `h` / `l`       | Scroll left / right            |
| `g` / `G`       | Go to top / bottom of page     |
| `b` / `f`       | Go back / forward in history   |
| `/`             | Real-time search with highlights |
| `n` / `N`       | Next / previous search match   |
| `i`             | List all images on the page     |
| `?`             | Show help overlay               |
| `Esc`           | Cancel current page load        |
| `Tab`           | Switch between content & URL bar |
| `Ctrl+L`        | List all links on the page      |
| `Ctrl+T`        | Open a new tab                  |
| `Ctrl+W`        | Close current tab               |
| `>`             | Switch to next tab              |
| `<`             | Switch to previous tab          |
| `Ctrl+S`        | Open settings                   |
| `Ctrl+P`        | Paste URL from clipboard        |
| `q` / `Ctrl+C`  | Quit                           |

### Settings Page

The settings page has a two-column layout: categories on the left, settings on the right. A word-wrapping description bar at the top of the right column shows the focused setting's help text.

| Key                 | Action                                   |
|---------------------|------------------------------------------|
| `Tab`               | Switch between categories / settings     |
| `↑` / `↓` (left)    | Select a category                        |
| `Enter` (category)  | Open the settings form on the right      |
| `↑` / `↓` (right)   | Move between fields (updates description)|
| `Ctrl+↑` / `Ctrl+↓` | Navigate between fields and buttons      |
| `Ctrl+S`            | Save settings and close                  |
| `Ctrl+Q`            | Close without saving                     |
| `Esc`               | Close settings                           |

---

## 🏗️ Architecture

### File Map

```
t-browser/
│
├── main.go                  # Entry point — creates a Browser and calls Run()
│
├── browser.go               # NewBrowser(), colour helpers, tab management, spinner, itemsPerPage
├── types.go                 # Browser struct, Tab struct, Link struct, constants
├── config.go                # Config struct, Viper initialisation, config I/O
├── theme.go                 # Dark/light theme definitions and application
│
├── session.go               # Session persistence, GoBack / GoForward
├── navigation.go            # NavigateTo(), URL validation (net.ParseIP SSRF), loading spinner
├── toast.go                 # Transient status-bar toast notifications
├── renderer.go              # Page rendering (readability, goquery, meta refresh, resolveURL)
├── html_renderer.go         # renderNode() — walks the HTML DOM tree, renderTable() with borders
├── links.go                 # extractVisibleLinks() — filters links by content
├── image.go                 # Image utilities (extension check, content-type)
├── image_preview.go         # Terminal image preview modal
│
├── search.go                # Search UI (input field, result list, n/N match navigation)
├── search_highlight.go      # Regex matching and text highlighting
│
├── http.go                  # HTTPClient, HTTP/2, gzip/deflate/brotli, ETag cache, Esc cancel
├── http_retry.go            # Transient retry, Retry-After backoff, cache TTL helpers
├── cookie.go                # Cookie struct, domain/path matching, persistence
│
├── content_security.go      # sanitizeHTML() + SanitizeReport, blockExternalResources()
├── security.go              # Certificate pinning, PinCurrentSiteKey(), HSTS store
├── url_security.go          # Tracking-param stripping, domain blocklist
│
├── ui.go                    # createUI(), setupKeyBindings(), resolveInputURL() (web search)
├── settings.go              # Settings data model and update logic
├── settings_modal.go        # Two-column settings modal UI
├── pagination.go            # Paginated link / image list modals with type-to-filter
├── keyutil.go               # isTabKey / isShiftTab helpers (terminal Tab key compat)
├── textutil.go              # wrapText — word-boundary text wrapping for UI
│
├── Makefile                 # Build, test, coverage, lint
│
├── *_test.go                          # Tests alongside each module
├── http_enhancements_test.go          # HTTP/2, brotli, deflate, caching, meta charset tests
├── http_retry_test.go                 # Transient retry, Retry-After, cache TTL tests
├── navigation_test.go                 # SSRF (net.ParseIP) validation tests
├── rendering_test.go                  # Table rendering, sanitisation report tests
├── toast_test.go                      # Spinner, status toast, tab-bar loading tests
├── ui_test.go                         # Web search resolution, tab management tests
├── bugfix_test.go                     # Cache eviction, cancel race, meta refresh cancel, deadlock tests
├── search_navigation_test.go          # n/N search match navigation tests
├── image_extraction_test.go           # Image extraction and preview tests
├── url_security_test.go              # Tracking-param stripping, domain blocklist tests
├── keyutil_test.go                   # isTabKey / isShiftTab tests
└── enhancements_test.go              # Pagination filters, heading colours, settings wiring tests
```

### How a Page Loads

```
User types URL ----> ui.go (resolveInputURL: URL or search query?)
                          │
                          ▼
                     navigation.go (NavigateTo)
                          │
                          ├── strip tracking params (utm_*/fbclid/gclid) if enabled
                          ├── cancel any pending meta-refresh goroutine
                          ├── validateAndSanitizeURL() — net.ParseIP SSRF + domain blocklist
                          │
                          ▼
                     http.go (FetchPage)
                          │
                          ├── cache TTL check (serve fresh entry without network)
                          ├── ETag/Last-Modified conditional cache check
                          ├── cookies sent with request
                          ├── Esc cancels via context.CancelFunc
                          ├── redirects handled manually (cookies forwarded)
                          ├── retry 429/502/503/504 with Retry-After backoff
                          ├── HSTS header processed
                          ├── gzip / deflate / brotli decompression
                          ├── binary detection + <meta charset> detection
                          └── charset conversion (Latin-1, ISO-8859-*, UTF-16)
                          │
                          ▼
                     renderer.go (renderPage)
                          ├── content_security.go: sanitizeHTML() + SanitizeReport
                          ├── meta refresh detection (cancellable goroutine)
                          ├── goquery: extract links & images (ResolveReference)
                          ├── content_security.go: blockExternalResources()
                          ├── go-readability: extract main article content
                          └── html_renderer.go: renderNode() / renderTable() for fallback
                          │
                          ▼
                     Tab.textView.SetText() — content displayed in terminal
```

### Module Groups

| Group                | Files |
|----------------------|-------|
| **Core**             | `main.go`, `browser.go`, `types.go`, `config.go`, `theme.go` |
| **Navigation**       | `navigation.go`, `session.go`, `toast.go` |
| **HTTP & Cookies**   | `http.go`, `http_retry.go`, `cookie.go` |
| **Rendering**        | `renderer.go`, `html_renderer.go`, `links.go`, `image.go`, `image_preview.go` |
| **Search**           | `search.go`, `search_highlight.go` |
| **Security**         | `content_security.go`, `security.go`, `url_security.go` |
| **UI**               | `ui.go`, `settings.go`, `settings_modal.go`, `pagination.go`, `keyutil.go`, `textutil.go` |
| **Tests**            | `*_test.go`, `http_enhancements_test.go`, `http_retry_test.go`, `navigation_test.go`, `rendering_test.go`, `toast_test.go`, `ui_test.go`, `bugfix_test.go`, `url_security_test.go`, `keyutil_test.go`, `enhancements_test.go` |

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
| `max_retries`               | int      | `3`                         | Network     | Retry attempts for 429/502/503/504 |
| `cache_ttl_seconds`         | int      | `0` (always revalidate)     | Network     | Client cache freshness lifetime |
| `enable_pinning`            | bool     | `false`                     | Network     | Certificate public-key pinning |
| `enable_hsts`               | bool     | `true`                      | Network     | HSTS (Strict-Transport-Security) |
| `blocked_domains`           | list     | —                           | Network     | Domains blocked at navigation (subdomain-matched) |
| `max_page_size`             | int      | `50` MB                     | Content     | Max response body |
| `max_image_size`            | int      | `5` MB                      | Content     | Max image file size |
| `enable_images`             | bool     | `true`                      | Content     | Load images from pages |
| `enable_content_security`   | bool     | `true`                      | Content     | Strip scripts, iframes, event handlers |
| `block_external_resources`  | bool     | `true`                      | Content     | Block cross-origin resources |
| `theme`                     | string   | `dark`                      | UI          | Colour theme |
| `show_images`               | bool     | `true`                      | UI          | Display images as ASCII |
| `word_wrap`                 | bool     | `true`                      | UI          | Soft-wrap long lines |
| `items_per_page`            | int      | `20`                        | UI          | Page size for links/images lists |
| `enable_cookies`            | bool     | `true`                      | Privacy     | Cookie storage |
| `cookie_auto_save`          | bool     | `true`                      | Privacy     | Persist cookies to disk |
| `enforce_same_site`         | bool     | `true`                      | Privacy     | SameSite=Strict enforcement |
| `strip_tracking_params`     | bool     | `true`                      | Privacy     | Remove utm_*/fbclid/gclid from URLs |
| `session_auto_save`         | bool     | `true`                      | Privacy     | Auto-save sessions |
| `search_engine`             | string   | `https://duckduckgo.com/html?q=` | UI      | URL prefix for web search |

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
| **Internal address block**| `localhost`, `127.*`, `10.*`, `192.168.*`, `172.16-31.*`, `169.254.*` (link-local), `::1` (IPv6), `0.0.0.0` |
| **Open redirect guard**   | Redirects must stay on the same host or a subdomain |
| **Content-type gate**     | Non-HTML responses (images, JSON, etc.) are rejected |
| **Binary magic check**    | PNG, JPEG, GIF, ZIP, PDF signatures detected and rejected |
| **Size limits**           | 50 MB pages, 5 MB images |
| **Charset conversion**    | Latin-1, ISO-8859-*, UTF-16 → UTF-8, plus `<meta charset>` detection |
| **tview injection**       | Brackets escaped to prevent format-code injection |
| **Whitespace normaliser** | Consecutive blank lines collapsed |
| **Sanitisation logging**  | Stripped scripts, iframes, and handlers logged to stderr for debugging |

### Configurable Defences

| Feature                   | Enabled by default? | Config key | How it works |
|---------------------------|:---:|------------|--------------|
| **HTML sanitisation**     | ✅ | `enable_content_security` | Strips `<script>`, `<iframe>`, `<object>`, `<embed>`, `<applet>`, all `on*` event handlers, and `javascript:`/`vbscript:` URLs from HTML before rendering |
| **External resource blocking** | ✅ | `block_external_resources` | Images, iframes, scripts, stylesheets, and `<source>` elements from other origins have their `src`/`href` removed before the page is rendered |
| **SameSite enforcement**  | ✅ | `enforce_same_site` | Cookies with `SameSite=Strict` are not sent on cross-domain requests. `SameSite=Lax` always allowed (all requests are navigations). `SameSite=None` always allowed. |
| **HSTS**                  | ✅ | `enable_hsts` | `Strict-Transport-Security` headers are parsed and cached per domain. HTTP requests to HSTS hosts are transparently upgraded to HTTPS. Policies persisted to `~/.config/t-browser/hsts/policies.json`. |
| **Tracking-param stripping** | ✅ | `strip_tracking_params` | Removes `utm_*`, `fbclid`, `gclid`, `msclkid`, `twclid`, `yclid`, and other analytics tracking parameters from the URL before navigation. A status toast notifies when params are stripped. |
| **Domain blocklist**      | ❌ | `blocked_domains` | A list of domains rejected at navigation time. Matching is case-insensitive and covers subdomains (e.g. `example.com` blocks `a.b.example.com`). Suffix-attack safe (`example.com.evil.com` is not blocked). |
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

### Blocking Domains

```yaml
# ~/.config/t-browser/config.yaml
blocked_domains:
  - "example.com"     # blocks example.com and *.example.com
  - "ads.tracker.test"
```

---

## 📝 Notes

- **No JavaScript engine** — SPAs and JS-heavy sites won't render as expected
- **Text-first display** — simplifies complex layouts into readable paragraphs
- **Fast & lightweight** — designed for minimal CPU/memory usage
- **Keyboard-only** — fully accessible from any terminal
- **HTTP/2 enabled** — automatic negotiation with HTTP/1.1 fallback
- **Brotli + gzip + deflate** — all three compression formats supported
- **Retry** — 429/502/503/504 retried with exponential backoff and `Retry-After` support
- **Cache TTL** — optional client-side freshness lifetime (`cache_ttl_seconds`); defaults to always-revalidate
- **Tables** — rendered as aligned text grids with Unicode box-drawing borders
- **Heading colours** — h1–h6 rendered in distinct colours (cyan/yellow/green/blue/magenta/red)
- **Meta refresh** — pages with `<meta http-equiv="refresh">` auto-navigate
- **Web search** — non-URL input in the URL bar is sent to your configured search engine
- **Link/image filter** — type to filter inside the links (`Ctrl+L`) and images (`i`) modals
- **Tracking-param stripping** — `utm_*`/`fbclid`/`gclid` etc. removed from navigated URLs
- **Domain blocklist** — configurable list of blocked domains with subdomain matching

---

## 🤝 Contributing

```bash
make test           # Run all tests (551 tests)
make test-race      # Run with race detector
make lint           # Run go vet
make coverage       # Check per-function coverage
```

All source files follow Go conventions. Tests live alongside the module they test (`*_test.go`).
