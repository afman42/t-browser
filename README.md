# Terminal Browser (t-browser)

A simple terminal-based web browser written in Go that allows you to browse websites directly from your terminal.

## Features

- **Web Browsing**: Fetch and display web pages in the terminal
- **Text Rendering**: Clean text rendering of HTML content with proper formatting
- **Navigation**: Back and forward navigation through browsing history
- **Search**: Real-time search functionality with match highlighting
- **Content Extraction**: Uses go-readability to extract main content (removes ads, menus, etc.)
- **Encoding Support**: Handles various text encodings (UTF-8, ISO-8859-1, etc.)
- **Proxy Support**: Configurable proxy support via environment variables
- **Cookie Handling**: Basic cookie storage and management
- **User Agent**: Spoofs user agent to get proper desktop/mobile versions

## Installation

1. Make sure you have Go 1.24+ installed
2. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/t-browser.git
   cd t-browser
   ```
3. Install dependencies:
   ```bash
   go mod tidy
   ```
4. Build the project:
   ```bash
   go build
   ```

## Usage

Run the browser:
```bash
./t-browser [optional-url]
```

Or to open a specific website:
```bash
./t-browser https://example.com
```

## Keyboard Controls

- `q` or `Ctrl+C`: Quit the browser
- `/`: Start real-time search
- `b`: Go back in history
- `f`: Go forward in history
- `Enter`: Follow highlighted link (not fully implemented)

## Search Functionality

The search feature provides real-time search with:
- Live highlighting of matches in yellow
- Match count display
- List of all matching words/phrases at the bottom
- Case-sensitive searching by default

## Configuration

- **Proxy**: Set the `PROXY` environment variable to use a proxy server
  ```bash
  export PROXY=http://your-proxy:port
  ./t-browser
  ```

## Dependencies

This project uses:
- `github.com/PuerkitoBio/goquery` - For HTML parsing
- `github.com/go-shiori/go-readability` - For content extraction
- `github.com/gdamore/tcell/v2` - For terminal UI
- `github.com/rivo/tview` - For terminal widgets
- `golang.org/x/net/html` - For HTML processing
- `golang.org/x/text` - For encoding support

## How It Works

1. **Fetching**: Uses Go's net/http to fetch web pages with proper headers and cookies
2. **Parsing**: Extracts and cleans HTML content
3. **Content Extraction**: Uses go-readability to get the main content, filtering out navigation, ads, etc.
4. **Rendering**: Converts HTML to formatted plain text for terminal display
5. **UI**: Provides a simple terminal interface for navigation and search

## Known Limitations

- Complex websites with heavy JavaScript may not render correctly
- Some CSS styling is lost in the conversion to plain text
- Link following is limited (work in progress)

## License

This project is available under the MIT License.