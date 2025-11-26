# Terminal Browser (t-browser)

A lightweight terminal-based web browser written in Go. Browse websites directly from your command line with minimal resource usage.

## ✨ Features

- **Browse in Terminal**: View web content directly in your terminal without GUI overhead
- **Smart Content Extraction**: Strips away ads, menus, and navigation to show main content only
- **Easy Navigation**: Move through pages with familiar keyboard shortcuts
- **Link Selection**: View all links in a clean modal list with 'l' key
- **Enhanced Search**: Find and highlight content as you type
- **Loading Indicators**: Animated progress indicator during page loads
- **Tab Navigation**: Switch between content view and URL input with Tab key
- **Security Features**: URL validation, redirect limits, and input sanitization
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
| `j` | Scroll down content |
| `k` | Scroll up content |
| `l` | Show modal list of all links on page |
| `Enter` | Confirm selection in modal |
| `?` | Show help screen |
| `Tab` | Switch between content view and URL input box |
| `p` | Paste URL from clipboard (when in URL input field) |

### Content Navigation
- Use `j` and `k` to scroll through content
- Long lines are not wrapped, adjust your terminal width for optimal viewing
- Content automatically scrolls to the top when a new page loads

### Link Navigation
- Press `l` to open a clean modal list of all links on the current page
- Each link shows both the link text and full URL for context
- Use arrow keys to navigate the list and Enter to select a link
- Includes a "Go Back" button to return to the previous page if available
- Press Escape or 'q' to close the modal and return to content

### Clipboard Support
- Press `p` when in the URL input field to paste content from clipboard
- Pasted URLs will replace current text in the input field
- Use arrow keys to navigate long pasted URLs if needed

### URL Input
- Type URLs in the input box at the bottom
- Press `Tab` to switch from content view to the URL input
- Press `Tab` again to switch back from URL input to content view
- For long URLs: use ← and → arrow keys to navigate within the input field
- The input field shows a portion of long URLs; use arrow keys to see more content
- Press `Enter` to navigate to the URL

### Search Functionality
- Press `/` to start search mode
- Type text to find in real-time
- Matching text is highlighted in yellow throughout the content
- Shows count of matches found

### Loading Indicators
- When loading content, an animated "Loading..." indicator appears
- Shows progress during page loading
- Automatically disappears when content is fully loaded

## ⚙️ Configuration

### Proxy Setup
Set a proxy server using environment variables:

```bash
export PROXY=http://your-proxy:port
./t-browser
```

## 🔐 Security Features

- **URL Validation**: Blocks dangerous schemes like `javascript:`, `data:`, etc.
- **Redirect Limits**: Prevents infinite redirect loops (max 10 redirects)
- **Local Address Blocking**: Prevents access to internal/local addresses
- **Input Sanitization**: Escapes formatting codes to prevent injection attacks
- **Content Security**: Sanitizes web content to prevent formatting code injection

## 🏗️ How It Works

1. **Request Processing**: Makes HTTP requests with appropriate headers and user agent
2. **Content Extraction**: Uses `go-readability` to extract main content while filtering out ads, menus, etc.
3. **Text Rendering**: Converts HTML to readable plain text with proper formatting
4. **Terminal UI**: Uses `tview` and `tcell` to create interactive terminal interface
5. **Link Management**: Identifies and presents all visible links in a modal list
6. **Loading Indicators**: Shows animated progress during page loads
7. **Focus Management**: Properly handles focus switching between UI components

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
- **Navigation**: Full keyboard-based navigation with intuitive shortcuts
- **Link Handling**: All visible links are presented in an organized modal list accessible with 'l' key

## 🤝 Contributing

Feel free to submit issues and enhancement requests! This project welcomes contributions from the community.

## 📄 License

MIT License - see the [LICENSE](LICENSE) file for details.