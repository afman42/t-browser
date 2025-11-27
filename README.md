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
- **Configure browser settings** through an intuitive settings page

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

| Key             | What it Does                               |
| --------------- | ------------------------------------------ |
| `q` or `Ctrl+C` | Exit the browser                           |
| `/`             | Find text on the page                      |
| `b`             | Go back to previous page                   |
| `f`             | Go forward (like browser history)          |
| `j`             | Move down through content                  |
| `k`             | Move up through content                    |
| `l`             | See all links on current page              |
| `i`             | See all images on current page             |
| `s`             | Open settings page                         |
| `Enter`         | Choose a link or image                     |
| `?`             | Show help                                  |
| `Tab`           | Switch between web content and address bar |
| `p`             | Paste a URL from clipboard                 |

### Settings Page Navigation

The settings page features a two-column layout with additional navigation options:

| Key Combination                                 | What it Does                                                        |
| ----------------------------------------------- | ------------------------------------------------------------------- |
| `s`                                             | Open settings page with category list on left and settings on right |
| `Tab`                                           | Switch focus between left (categories) and right (settings) columns |
| `Shift+Tab`                                     | Navigate between form elements in the right column                  |
| `Tab` (in left column, when right is not empty) | Switch to the settings form                                         |
| `Cancel button`                                 | Clear the right column and set focus to left column                 |

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

## ⚙️ Configuration and Settings

The browser uses a configuration file to manage settings across sessions and provides an in-browser settings page for easy configuration. The configuration file is automatically created and managed in the appropriate OS-specific location:

- **Linux**: `~/.config/t-browser/config.yaml`
- **Windows**: `%APPDATA%\t-browser\config.yaml`
- **macOS**: `~/Library/Application Support/t-browser/config.yaml`

### Settings Page

Access the settings page by pressing the `s` key while browsing. The settings page features:

- **Two-column layout**: Categories on the left, settings on the right
- **Six categories**: Browser, Network, Content, UI, Privacy, and Advanced settings
- **Intuitive navigation**: Use Tab to switch between columns, Alt+Arrows to navigate form elements
- **Visual feedback**: Save and Cancel buttons appear when changes are made
- **Cancel functionality**: Press Cancel to clear the right column and return focus to categories
- **Persistent settings**: Changes are saved to the configuration file when you Save

### Configuration Settings

The configuration file includes the following settings:

- `user_agent`: Custom user agent string for HTTP requests
- `cookie_file`: Path to store persistent cookies
- `cookie_auto_save`: Enable automatic cookie saving
- `session_file`: Path to store session data
- `session_auto_save`: Enable automatic session saving
- `proxy`: Proxy server URL (e.g., "http://proxy:port")
- `request_timeout`: Time in seconds before request timeout
- `max_redirects`: Maximum number of HTTP redirects to follow
- `max_page_size`: Maximum size (in bytes) for downloaded pages
- `max_image_size`: Maximum size (in bytes) for images
- `enable_images`: Enable or disable image loading
- `enable_cookies`: Enable or disable cookie storage
- `theme`: Color theme for the interface
- `show_images`: Display images in terminal
- `word_wrap`: Enable text word wrapping

### Proxy Setup

You can set a proxy using the configuration file or environment variable:

Using the config file (recommended):

```yaml
proxy: http://your-proxy:port
```

Or use environment variable:

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
- **Config security**: Configuration and cookie files stored securely with appropriate permissions

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
