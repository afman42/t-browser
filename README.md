# Terminal Browser (t-browser)

A lightweight terminal-based web browser written in Go. Browse websites directly from your command line with minimal resource usage.

## ✨ Features

- **Browse in Terminal**: View web content directly in your terminal without GUI overhead
- **Smart Content Extraction**: Strips away ads, menus, and navigation to show main content only
- **Easy Navigation**: Move through pages with familiar keyboard shortcuts
- **Link Selection**: Navigate and select links using the keyboard
- **Real-time Search**: Find and highlight content as you type
- **Multiple Encodings**: Handles UTF-8, Latin-1, and other character encodings
- **Proxy Support**: Works with proxy servers for network access

## 🚀 Quick Start

### Prerequisites
- Go 1.19+ installed on your system

### Installation
```bash
# Clone the repository
git clone https://github.com/yourusername/t-browser.git
cd t-browser

# Install dependencies
go mod tidy

# Build the executable
go build
```

### Usage
```bash
# Launch with default page
./t-browser

# Open a specific website
./t-browser https://example.com

# Open a website (with or without https:// prefix)
./t-browser example.com
```

## ⌨️ Keyboard Controls

| Key | Action |
|-----|--------|
| `q` or `Ctrl+C` | Quit the browser |
| `/` | Start search mode |
| `b` | Go back in history |
| `f` | Go forward in history |
| `j` | Move to next link |
| `k` | Move to previous link |
| `Enter` | Follow the selected link |
| `?` | Show help screen |

### Link Navigation
- Links are automatically numbered in the text (e.g., `click here [1]`)
- Use `j` and `k` to move between links
- The current link is highlighted in blue
- Press `Enter` to visit the selected link

### Search Functionality
- Type in search box to find text in real-time
- Matching text is highlighted in yellow
- Shows count of matches found
- Case-sensitive by default

## ⚙️ Configuration

### Proxy Setup
Set a proxy server using environment variables:

```bash
export PROXY=http://your-proxy:port
./t-browser
```

## 🏗️ How It Works

1. **Request Processing**: Makes HTTP requests with appropriate headers and user agent
2. **Content Extraction**: Uses `go-readability` to extract main content while filtering out ads, menus, etc.
3. **Text Rendering**: Converts HTML to readable plain text with proper formatting
4. **Terminal UI**: Uses `tview` and `tcell` to create interactive terminal interface
5. **Link Management**: Identifies and numbers links for easy keyboard navigation

## 🔧 Dependencies

- `github.com/PuerkitoBio/goquery` - HTML parsing and manipulation
- `github.com/go-shiori/go-readability` - Content extraction from web pages
- `github.com/gdamore/tcell/v2` - Terminal interface handling
- `github.com/rivo/tview` - Terminal widgets and UI components
- `golang.org/x/net/html` - HTML processing utilities
- `golang.org/x/text` - Character encoding support

## 📝 Notes

- **JavaScript**: Since content is rendered as plain text, JavaScript-heavy sites may not display correctly
- **Visual Elements**: Images and complex layouts are simplified to text format
- **Performance**: Lightweight and fast, suitable for low-resource environments

## 🤝 Contributing

Feel free to submit issues and enhancement requests! This project welcomes contributions from the community.

## 📄 License

MIT License - see the [LICENSE](LICENSE) file for details.