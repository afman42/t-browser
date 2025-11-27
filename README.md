# Terminal Browser (t-browser)

A lightweight web browser that runs directly in your terminal. Browse the web without opening a separate window - just use your keyboard!

## ✨ What You Can Do

- **Browse websites** directly from your terminal
- **View main content** while hiding ads and menus
- **Navigate with keyboard** shortcuts (no mouse needed)
- **See all links** on a page with one command
- **Preview images** right in your terminal
- **Search text** on any webpage instantly
- **Use basic proxy** for connections

## 🚀 How to Get Started

### What You Need
- Go programming language (version 1.19 or newer)

### Install and Run
```bash
# Download the code
git clone https://github.com/yourusername/t-browser.git
cd t-browser

# Get required packages
go mod tidy

# Build the program
go build

# Run the browser
./t-browser
```

### Quick Usage Examples
```bash
# Open with default homepage
./t-browser

# Go to a specific website
./t-browser https://example.com

# Or just type the website name (https is added automatically)
./t-browser example.com
```

## ⌨️ Keyboard Shortcuts

| Key | What it Does |
|-----|--------------|
| `q` or `Ctrl+C` | Exit the browser |
| `/` | Find text on the page |
| `b` | Go back to previous page |
| `f` | Go forward (like browser history) |
| `j` | Move down through content |
| `k` | Move up through content |
| `l` | See all links on current page |
| `i` | See all images on current page |
| `Enter` | Choose a link or image |
| `?` | Show help |
| `Tab` | Switch between web content and address bar |
| `p` | Paste a URL from clipboard |

### Browsing Tips
- Use `j` and `k` to move up/down the webpage
- Long text lines won't wrap - make your terminal wider if needed
- New pages always start at the top

### Finding Links
- Press `l` to see a clean list of all links
- Each link shows the text and its full web address
- Links with images show [IMAGE] or [IMAGE*] markers
- Use arrow keys to pick a link, then press Enter
- Press Esc or 'q' to return to the page

### Viewing Images
- Press `i` to see all images on the page
- Shows image name, description, and file type
- Pick any image to view it in your terminal
- Supports JPG, PNG, GIF, BMP, WebP, and more
- Files limited to 5MB to save space and time
- Press Esc or 'q' to return

### Search on Any Page
- Press `/` to start searching
- Type any words - they light up in yellow
- See how many matches you found

## ⚙️ Using a Proxy

If you need to use a proxy, set it with:
```bash
export PROXY=http://your-proxy:port
./t-browser
```

## 🔐 Safety Features

- **URL checking**: Blocks dangerous web addresses
- **Redirect protection**: Stops endless redirects
- **Local blocking**: Won't access internal computers
- **Size limits**: Images limited to 5MB
- **Content cleaning**: Strips harmful formatting

## 🏗️ How It Works

1. **Fetch**: Gets web pages from the internet
2. **Clean**: Removes ads, menus, and clutter
3. **Show**: Displays main content in your terminal
4. **Handle**: Processes your keyboard commands
5. **Preview**: Shows images directly in text format

## 📝 Important Notes

- **No JavaScript**: Complex interactive sites might not work right
- **Text focused**: Complex layouts become simple text
- **Fast**: Uses very little computer power
- **Keyboard only**: Full control with just your keys

## 🤝 Help Make It Better

Found a problem or want to suggest something? Report it on GitHub!

## 📄 License

This project is open source under the MIT License.