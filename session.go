package main

import (
	"encoding/json"
	"os"
)

// Session represents the state of a browser session
// (defined in types.go, referenced here for documentation)
type Session struct {
	History      []string `json:"history"`
	HistoryIndex int      `json:"history_index"`
	CurrentURL   string   `json:"current_url"`
	SearchTerm   string   `json:"search_term"`
	ForceUA      string   `json:"force_ua"`
}

// SaveSession saves the current browser state to a file
func (b *Browser) SaveSession(filename string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	session := &Session{
		History:      b.currentTab().history,
		HistoryIndex: b.currentTab().historyIndex,
		CurrentURL:   b.currentTab().currentURL,
		SearchTerm:   b.currentTab().searchTerm,
		ForceUA:      b.forceUA,
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0o600)
}

// LoadSession loads a browser state from a file
func (b *Browser) LoadSession(filename string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var session Session
	err = json.Unmarshal(data, &session)
	if err != nil {
		return err
	}

	b.currentTab().history = session.History
	b.currentTab().historyIndex = session.HistoryIndex
	b.currentTab().currentURL = session.CurrentURL
	b.currentTab().searchTerm = session.SearchTerm
	b.forceUA = session.ForceUA
	// Apply the restored UA to the live client; b.forceUA alone is never read.
	if b.client != nil && session.ForceUA != "" {
		b.client.SetUserAgent(session.ForceUA)
	}

	return nil
}

// GoBack navigates back in history.
// The mutex is released before calling NavigateTo because it may
// block on I/O and must not be held during that call.
func (b *Browser) GoBack() {
	b.mu.Lock()
	if b.currentTab().historyIndex <= 0 {
		b.mu.Unlock()
		return
	}
	b.currentTab().historyIndex--
	url := b.currentTab().history[b.currentTab().historyIndex]
	b.mu.Unlock()
	b.NavigateTo(url)
}

// GoForward navigates forward in history.
// The mutex is released before calling NavigateTo because it may
// block on I/O and must not be held during that call.
func (b *Browser) GoForward() {
	b.mu.Lock()
	if b.currentTab().historyIndex >= len(b.currentTab().history)-1 {
		b.mu.Unlock()
		return
	}
	b.currentTab().historyIndex++
	url := b.currentTab().history[b.currentTab().historyIndex]
	b.mu.Unlock()
	b.NavigateTo(url)
}
