package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// filterLinksBy returns the subset of links whose text or URL contains the
// given filter text (case-insensitive).  An empty filter returns the input
// slice unchanged.  This is pure/testable logic used by the links modal.
func filterLinksBy(links []Link, filterText string) []Link {
	filterText = strings.ToLower(strings.TrimSpace(filterText))
	if filterText == "" {
		return links
	}
	var out []Link
	for _, l := range links {
		if strings.Contains(strings.ToLower(l.Text), filterText) ||
			strings.Contains(strings.ToLower(l.URL), filterText) {
			out = append(out, l)
		}
	}
	return out
}

// filterImagesBy returns the subset of images whose Alt/Title/URL contains the
// given filter text (case-insensitive).  An empty filter returns the input
// slice unchanged.
func filterImagesBy(images []Image, filterText string) []Image {
	filterText = strings.ToLower(strings.TrimSpace(filterText))
	if filterText == "" {
		return images
	}
	var out []Image
	for _, img := range images {
		if strings.Contains(strings.ToLower(img.Alt), filterText) ||
			strings.Contains(strings.ToLower(img.Title), filterText) ||
			strings.Contains(strings.ToLower(img.URL), filterText) {
			out = append(out, img)
		}
	}
	return out
}

// closeModalToMain restores the main content view, used by the link/image
// modals when the user closes them or activates a navigable entry.
func (b *Browser) closeModalToMain() {
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.currentTab().textView, 0, 1, false).
		AddItem(b.urlInput, 3, 0, false)
	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.currentTab().textView)
}

// showLinksModal displays a modal with a list of all links on the page with
// pagination and an optional type-to-filter input.
func (b *Browser) showLinksModal() {
	if len(b.currentTab().links) == 0 {
		return
	}
	b.showLinksModalPage(0)
}

// showLinksModalPage displays a specific page of links in the modal.  The
// modal is built once; pagination and filtering re-render the list in place
// via the refresh closure.
func (b *Browser) showLinksModalPage(page int) {
	if len(b.currentTab().links) == 0 {
		return
	}

	ipp := b.itemsPerPage()
	allLinks := b.currentTab().links

	// Show a brief loading indicator for very large lists, preserving the
	// original behaviour.
	if len(allLinks) > 50 {
		b.showLoadingModal("Loading Links", "[yellow]Loading links...[white]")
	}

	inputField := tview.NewInputField().SetLabel("Filter: ")
	inputField.SetFieldBackgroundColor(tcell.ColorDefault)

	linkList := tview.NewList()
	linkList.SetBorder(true)
	linkList.ShowSecondaryText(true)

	currentPage := page

	var refresh func()
	refresh = func() {
		filtered := filterLinksBy(allLinks, inputField.GetText())
		totalItems := len(filtered)
		totalPages := (totalItems + ipp - 1) / ipp
		if totalPages == 0 {
			totalPages = 1
		}
		if currentPage < 0 {
			currentPage = 0
		}
		if currentPage >= totalPages {
			currentPage = totalPages - 1
		}
		startIndex := currentPage * ipp
		endIndex := startIndex + ipp
		if endIndex > totalItems {
			endIndex = totalItems
		}

		linkList.Clear()
		if totalItems == 0 {
			linkList.SetTitle("Links (no matches)")
			linkList.AddItem("No links match filter", "", 0, nil)
		} else {
			linkList.SetTitle(fmt.Sprintf("Links (Page %d of %d, %d matches)", currentPage+1, totalPages, totalItems))
			for i := startIndex; i < endIndex; i++ {
				l := filtered[i]
				linkText := l.Text
				// Extension-only detection.  Content-type probing would fire a
				// synchronous network HEAD per link on the UI thread and leak
				// every destination URL to its server.
				isImage := b.hasRealImageExtension(l.URL)
				if isImage {
					linkText += " [IMAGE*]"
				}
				urlToShow := l.URL
				if len(urlToShow) > 70 {
					urlToShow = urlToShow[:70] + "..."
				}
				linkList.AddItem(linkText, urlToShow, 0, func(target string, img bool) func() {
					return func() {
						if img {
							b.showImagePreview(target)
						} else {
							b.NavigateTo(target)
							b.closeModalToMain()
						}
					}
				}(l.URL, isImage))
			}

			if totalPages > 1 {
				if currentPage > 0 {
					linkList.AddItem("Previous Page", fmt.Sprintf("Go to page %d", currentPage), 0, func() {
						currentPage--
						refresh()
					})
				}
				if currentPage < totalPages-1 {
					linkList.AddItem("Next Page", fmt.Sprintf("Go to page %d", currentPage+2), 0, func() {
						currentPage++
						refresh()
					})
				}
			}
		}

		linkList.AddItem("Close", "Close the links list", 'c', func() {
			b.closeModalToMain()
		})

		if len(b.currentTab().images) > 0 {
			linkList.AddItem("Show Images", fmt.Sprintf("View all %d images on this page", len(b.currentTab().images)), 'i', func() {
				b.showImagesModal()
			})
		}

		if b.currentTab().historyIndex > 0 {
			linkList.AddItem("Go Back", "Return to previous page", 'b', func() {
				b.GoBack()
			})
		}
	}

	inputField.SetChangedFunc(func(text string) {
		currentPage = 0
		refresh()
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB, tcell.KeyDown:
			b.app.SetFocus(linkList)
			return nil
		case tcell.KeyEscape:
			b.closeModalToMain()
			return nil
		}
		return event
	})

	linkList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			b.closeModalToMain()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				b.closeModalToMain()
				return nil
			case '/':
				b.app.SetFocus(inputField)
				return nil
			}
		}
		return event
	})

	refresh()

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 3, 0, true).
		AddItem(linkList, 0, 1, false)
	b.app.SetRoot(flex, true)
	b.app.SetFocus(inputField)
}

// showImagesModal displays a modal with a list of all images on the page with
// pagination and an optional type-to-filter input.
func (b *Browser) showImagesModal() {
	if len(b.currentTab().images) == 0 {
		return
	}
	b.showImagesModalPage(0)
}

// showImagesModalPage displays a specific page of images in the modal.
func (b *Browser) showImagesModalPage(page int) {
	if len(b.currentTab().images) == 0 {
		return
	}

	ipp := b.itemsPerPage()
	allImages := b.currentTab().images

	if len(allImages) > 50 {
		b.showLoadingModal("Loading Images", "[yellow]Loading images...[white]")
	}

	inputField := tview.NewInputField().SetLabel("Filter: ")
	inputField.SetFieldBackgroundColor(tcell.ColorDefault)

	imageList := tview.NewList()
	imageList.SetBorder(true)
	imageList.ShowSecondaryText(true)

	currentPage := page

	var refresh func()
	refresh = func() {
		filtered := filterImagesBy(allImages, inputField.GetText())
		totalItems := len(filtered)
		totalPages := (totalItems + ipp - 1) / ipp
		if totalPages == 0 {
			totalPages = 1
		}
		if currentPage < 0 {
			currentPage = 0
		}
		if currentPage >= totalPages {
			currentPage = totalPages - 1
		}
		startIndex := currentPage * ipp
		endIndex := startIndex + ipp
		if endIndex > totalItems {
			endIndex = totalItems
		}

		imageList.Clear()
		if totalItems == 0 {
			imageList.SetTitle("Images (no matches)")
			imageList.AddItem("No images match filter", "", 0, nil)
		} else {
			imageList.SetTitle(fmt.Sprintf("Images (Page %d of %d, %d matches)", currentPage+1, totalPages, totalItems))
			for i := startIndex; i < endIndex; i++ {
				img := filtered[i]
				imgTitle := img.Alt
				if imgTitle == "" {
					imgTitle = fmt.Sprintf("Image %d", i+1)
				}
				ext := "unknown"
				if lastDot := strings.LastIndex(img.URL, "."); lastDot != -1 && lastDot < len(img.URL)-1 {
					ext = strings.ToLower(img.URL[lastDot+1:])
					if qIdx := strings.Index(ext, "?"); qIdx != -1 {
						ext = ext[:qIdx]
					}
				}
				urlToShow := fmt.Sprintf("%s [%s]", img.URL, ext)
				if len(urlToShow) > 70 {
					urlToShow = urlToShow[:70] + "..."
				}
				imageList.AddItem(imgTitle, urlToShow, 0, func(target string) func() {
					return func() {
						b.showImagePreview(target)
					}
				}(img.URL))
			}

			if totalPages > 1 {
				if currentPage > 0 {
					imageList.AddItem("Previous Page", fmt.Sprintf("Go to page %d", currentPage), 0, func() {
						currentPage--
						refresh()
					})
				}
				if currentPage < totalPages-1 {
					imageList.AddItem("Next Page", fmt.Sprintf("Go to page %d", currentPage+2), 0, func() {
						currentPage++
						refresh()
					})
				}
			}
		}

		imageList.AddItem("Close", "Close the images list", 'c', func() {
			b.closeModalToMain()
		})
		imageList.AddItem("View Links", "Return to links list", 'l', func() {
			b.showLinksModal()
		})
	}

	inputField.SetChangedFunc(func(text string) {
		currentPage = 0
		refresh()
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB, tcell.KeyDown:
			b.app.SetFocus(imageList)
			return nil
		case tcell.KeyEscape:
			b.closeModalToMain()
			return nil
		}
		return event
	})

	imageList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			b.closeModalToMain()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				b.closeModalToMain()
				return nil
			case '/':
				b.app.SetFocus(inputField)
				return nil
			}
		}
		return event
	})

	refresh()

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 3, 0, true).
		AddItem(imageList, 0, 1, false)
	b.app.SetRoot(flex, true)
	b.app.SetFocus(inputField)
}
