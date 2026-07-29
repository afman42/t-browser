package main

import (
	"fmt"
	"os"
)

func main() {
	browser := NewBrowser()

	if err := browser.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running browser: %v\n", err)
		os.Exit(1)
	}
}
