package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showLinksModal displays a modal with a list of all links on the page with pagination
func (b *Browser) showLinksModal() {
	if len(b.currentTab().links) == 0 {
		return
	}

	// Show the first page of links
	b.showLinksModalPage(0)
}

// showLinksModalPage displays a specific page of links in the modal
func (b *Browser) showLinksModalPage(page int) {
	if len(b.currentTab().links) == 0 {
		return
	}

	// Calculate pagination
	totalItems := len(b.currentTab().links)
	totalPages := (totalItems + ItemsPerPage - 1) / ItemsPerPage // Ceiling division
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	// Calculate start and end indices for this page
	startIndex := page * ItemsPerPage
	endIndex := startIndex + ItemsPerPage
	if endIndex > totalItems {
		endIndex = totalItems
	}

	// Show loading indicator first if there are many links
	if len(b.currentTab().links) > 50 { // Only show loading for larger lists
		b.showLoadingModal("Loading Links", fmt.Sprintf("[yellow]Loading links page %d of %d...[white]", page+1, totalPages))
	}

	// Create a new list for links
	linkList := tview.NewList()
	linkList.SetBorder(true)
	linkList.SetTitle(fmt.Sprintf("Links on this page (Page %d of %d)", page+1, totalPages))
	linkList.ShowSecondaryText(true)

	// Add links for this page
	for i := startIndex; i < endIndex; i++ {
		link := b.currentTab().links[i]
		linkText := link.Text
		// Don't truncate text when there are many links, just display as is
		// Check if the link is an image
		isImage := b.isImageURL(link.URL)
		hasRealExt := b.hasRealImageExtension(link.URL)
		if isImage {
			if hasRealExt {
				linkText += " [IMAGE*]" // Indicate that this is an image with real extension
			} else {
				linkText += " [IMAGE]" // Indicate that this is an image (detected by content type)
			}
		}

		// Format the URL to show just the domain and path, truncating long paths
		urlToShow := link.URL
		if len(urlToShow) > 70 {
			urlToShow = urlToShow[:70] + "..."
		}

		// Add the item with primary text as link text and secondary as URL
		linkList.AddItem(linkText, urlToShow, 0, func(index int, isImg bool, hasExt bool) func() {
			return func() {
				if isImg {
					// Show image preview instead of navigating
					b.showImagePreview(b.currentTab().links[index].URL)
				} else {
					// Navigate to the selected link
					b.NavigateTo(b.currentTab().links[index].URL)
					// Close the modal by returning to main view
					flex := tview.NewFlex().
						SetDirection(tview.FlexRow).
						AddItem(b.currentTab().textView, 0, 1, false).
						AddItem(b.urlInput, 3, 0, false)
					b.app.SetRoot(flex, true)
					b.app.SetFocus(b.currentTab().textView)
				}
			}
		}(i, isImage, hasRealExt))
	}

	// Add pagination controls if there are multiple pages
	if totalPages > 1 {
		// Add previous page button if not on the first page
		if page > 0 {
			linkList.AddItem("Previous Page", fmt.Sprintf("Go to page %d", page), 'p', func() {
				b.showLinksModalPage(page - 1)
			})
		}

		// Add next page button if not on the last page
		if page < totalPages-1 {
			linkList.AddItem("Next Page", fmt.Sprintf("Go to page %d", page+2), 'n', func() {
				b.showLinksModalPage(page + 1)
			})
		}
	}

	// Add a close option
	linkList.AddItem("Close", "Close the links list", 'c', func() {
		// Close the modal by returning to main view
		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(b.currentTab().textView, 0, 1, false).
			AddItem(b.urlInput, 3, 0, false)
		b.app.SetRoot(flex, true)
		b.app.SetFocus(b.currentTab().textView)
	})

	// Add an images option if there are images on the page
	if len(b.currentTab().images) > 0 {
		linkList.AddItem("Show Images", fmt.Sprintf("View all %d images on this page", len(b.currentTab().images)), 'i', func() {
			b.showImagesModal()
		})
	}

	// Add a go back option if there's history
	if b.currentTab().historyIndex > 0 {
		linkList.AddItem("Go Back", "Return to previous page", 'b', func() {
			// Go back in history and return to main view
			b.GoBack()
			// The GoBack function will handle updating the UI
		})
	}

	// Set up key handling for the modal
	linkList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			fallthrough
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				// Close the modal by returning to main view
				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.currentTab().textView, 0, 1, false).
					AddItem(b.urlInput, 3, 0, false)
				b.app.SetRoot(flex, true)
				b.app.SetFocus(b.currentTab().textView)
				return nil
			} else if event.Rune() == 'n' && totalPages > 1 && page < totalPages-1 {
				// Go to next page
				b.showLinksModalPage(page + 1)
				return nil
			} else if event.Rune() == 'p' && totalPages > 1 && page > 0 {
				// Go to previous page
				b.showLinksModalPage(page - 1)
				return nil
			}
		}
		return event
	})

	// Set the list as root (this removes the loading indicator)
	b.app.SetRoot(linkList, true)
}

// showImagesModal displays a modal with a list of all images on the page with pagination
func (b *Browser) showImagesModal() {
	if len(b.currentTab().images) == 0 {
		return
	}

	// Show the first page of images
	b.showImagesModalPage(0)
}

// showImagesModalPage displays a specific page of images in the modal
func (b *Browser) showImagesModalPage(page int) {
	if len(b.currentTab().images) == 0 {
		return
	}

	// Calculate pagination
	totalItems := len(b.currentTab().images)
	totalPages := (totalItems + ItemsPerPage - 1) / ItemsPerPage // Ceiling division
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	// Calculate start and end indices for this page
	startIndex := page * ItemsPerPage
	endIndex := startIndex + ItemsPerPage
	if endIndex > totalItems {
		endIndex = totalItems
	}

	// Show loading indicator first if there are many images
	if len(b.currentTab().images) > 50 { // Only show loading for larger lists
		b.showLoadingModal("Loading Images", fmt.Sprintf("[yellow]Loading images page %d of %d...[white]", page+1, totalPages))
	}

	// Create a new list for images
	imageList := tview.NewList()
	imageList.SetBorder(true)
	imageList.SetTitle(fmt.Sprintf("Images on this page (Page %d of %d)", page+1, totalPages))
	imageList.ShowSecondaryText(true)

	// Add images for this page
	for i := startIndex; i < endIndex; i++ {
		img := b.currentTab().images[i]

		// Create title for the image
		imgTitle := img.Alt
		if imgTitle == "" {
			// If no alt text, use a generic description
			imgTitle = fmt.Sprintf("Image %d", i+1)
		}

		// Don't truncate text when there are many images, just display as is
		// Extract the file extension from the URL
		ext := "unknown"
		lastDot := strings.LastIndex(img.URL, ".")
		if lastDot != -1 && lastDot < len(img.URL)-1 {
			ext = strings.ToLower(img.URL[lastDot+1:])
			// Handle query parameters that might follow the extension
			if queryIndex := strings.Index(ext, "?"); queryIndex != -1 {
				ext = ext[:queryIndex]
			}
		}

		// Format the image URL to show in secondary text with file extension, truncating long URLs
		urlToShow := fmt.Sprintf("%s [%s]", img.URL, ext)
		if len(urlToShow) > 70 {
			urlToShow = urlToShow[:70] + "..."
		}

		// Add the item with primary text as image title and secondary as URL with extension
		imageList.AddItem(imgTitle, urlToShow, 0, func(index int) func() {
			return func() {
				// Show image preview
				b.showImagePreview(b.currentTab().images[index].URL)
			}
		}(i))
	}

	// Add pagination controls if there are multiple pages
	if totalPages > 1 {
		// Add previous page button if not on the first page
		if page > 0 {
			imageList.AddItem("Previous Page", fmt.Sprintf("Go to page %d", page), 'p', func() {
				b.showImagesModalPage(page - 1)
			})
		}

		// Add next page button if not on the last page
		if page < totalPages-1 {
			imageList.AddItem("Next Page", fmt.Sprintf("Go to page %d", page+2), 'n', func() {
				b.showImagesModalPage(page + 1)
			})
		}
	}

	// Add a close option
	imageList.AddItem("Close", "Close the images list", 'c', func() {
		// Close the modal by returning to main view
		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(b.currentTab().textView, 0, 1, false).
			AddItem(b.urlInput, 3, 0, false)
		b.app.SetRoot(flex, true)
		b.app.SetFocus(b.currentTab().textView)
	})

	// Add a link list option to return to the links modal
	imageList.AddItem("View Links", "Return to links list", 'l', func() {
		b.showLinksModal()
	})

	// Set up key handling for the modal
	imageList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			fallthrough
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				// Close the modal by returning to main view
				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.currentTab().textView, 0, 1, false).
					AddItem(b.urlInput, 3, 0, false)
				b.app.SetRoot(flex, true)
				b.app.SetFocus(b.currentTab().textView)
				return nil
			} else if event.Rune() == 'n' && totalPages > 1 && page < totalPages-1 {
				// Go to next page
				b.showImagesModalPage(page + 1)
				return nil
			} else if event.Rune() == 'p' && totalPages > 1 && page > 0 {
				// Go to previous page
				b.showImagesModalPage(page - 1)
				return nil
			}
		}
		return event
	})

	// Set the list as root (this removes the loading indicator)
	b.app.SetRoot(imageList, true)
}
