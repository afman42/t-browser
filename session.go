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
		History:      b.history,
		HistoryIndex: b.historyIndex,
		CurrentURL:   b.currentURL,
		SearchTerm:   b.searchTerm,
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

	b.history = session.History
	b.historyIndex = session.HistoryIndex
	b.currentURL = session.CurrentURL
	b.searchTerm = session.SearchTerm
	b.forceUA = session.ForceUA

	return nil
}

// GoBack navigates back in history.
// The mutex is released before calling NavigateTo because it may
// block on I/O and must not be held during that call.
func (b *Browser) GoBack() {
	b.mu.Lock()
	if b.historyIndex <= 0 {
		b.mu.Unlock()
		return
	}
	b.historyIndex--
	url := b.history[b.historyIndex]
	b.mu.Unlock()
	b.NavigateTo(url)
}

// GoForward navigates forward in history.
// The mutex is released before calling NavigateTo because it may
// block on I/O and must not be held during that call.
func (b *Browser) GoForward() {
	b.mu.Lock()
	if b.historyIndex >= len(b.history)-1 {
		b.mu.Unlock()
		return
	}
	b.historyIndex++
	url := b.history[b.historyIndex]
	b.mu.Unlock()
	b.NavigateTo(url)
}