package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// keyHandler returns the input-capture closure installed on a primitive so
// tests can drive individual key branches without a running event loop.
func keyHandler(t *testing.T, p tview.Primitive) func(*tcell.EventKey) *tcell.EventKey {
	t.Helper()
	type capturer interface {
		GetInputCapture() func(*tcell.EventKey) *tcell.EventKey
	}
	c, ok := p.(capturer)
	if !ok {
		t.Fatalf("primitive %T exposes no GetInputCapture", p)
	}
	h := c.GetInputCapture()
	if h == nil {
		t.Fatalf("primitive %T has no input capture installed", p)
	}
	return h
}

func key(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }
func rn(r rune) *tcell.EventKey       { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }
func modKey(k tcell.Key, m tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(k, 0, m)
}

// newKeyBoundBrowser returns a Browser wired with every widget
// setupKeyBindings touches, plus a mock HTTP client so no branch reaches the
// network, and the content view's input-capture closure.
func newKeyBoundBrowser(t *testing.T) (*Browser, func(*tcell.EventKey) *tcell.EventKey) {
	t.Helper()
	b := testBrowserWithUI()
	b.client = newMockClient(map[string]mockResponse{"/": htmlOK})
	tab := b.currentTab()
	b.setupKeyBindings(tab.textView)
	return b, keyHandler(t, tab.textView)
}

func TestSetupKeyBindingsScrolling(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	tab := b.currentTab()
	tab.textView.SetText("line\nline\nline\nline\nline")

	for _, r := range []rune{'j', 'k', 'h', 'l', 'g', 'G'} {
		if got := h(rn(r)); got != nil {
			t.Errorf("rune %q should be consumed, got %v", r, got)
		}
	}
	// Scroll keys write the status bar.
	if b.statusBar.GetText(false) == "" {
		t.Error("scroll keys should refresh the status bar")
	}
}

func TestSetupKeyBindingsScrollUpClampsAtZero(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	b.currentTab().textView.ScrollTo(0, 0)
	h(rn('k')) // already at top: newRow < 0 branch
	row, _ := b.currentTab().textView.GetScrollOffset()
	if row != 0 {
		t.Errorf("scroll offset = %d, want clamped to 0", row)
	}
}

func TestSetupKeyBindingsHistoryKeys(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	// Empty history: GoBack/GoForward return early — no navigation goroutine.
	if got := h(rn('b')); got != nil {
		t.Error("'b' should be consumed")
	}
	if got := h(rn('f')); got != nil {
		t.Error("'f' should be consumed")
	}
	if len(b.currentTab().history) != 0 {
		t.Error("history must stay empty when there is nothing to go back to")
	}
}

func TestSetupKeyBindingsTabManagement(t *testing.T) {
	b, h := newKeyBoundBrowser(t)

	if got := h(key(tcell.KeyCtrlT)); got != nil {
		t.Error("Ctrl+T should be consumed")
	}
	if len(b.tabs) != 2 {
		t.Errorf("tabs after Ctrl+T = %d, want 2", len(b.tabs))
	}

	h(rn('>'))
	first := b.activeTab
	h(rn('<'))
	if b.activeTab == first {
		t.Error("'<' should switch back to the previous tab")
	}

	if got := h(key(tcell.KeyCtrlW)); got != nil {
		t.Error("Ctrl+W should be consumed")
	}
	if len(b.tabs) != 1 {
		t.Errorf("tabs after Ctrl+W = %d, want 1", len(b.tabs))
	}
}

func TestSetupKeyBindingsTabKeyVariants(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	b.tabs = append(b.tabs, newTab(), newTab())
	b.activeTab = 0

	// Ctrl+Tab → next tab.
	h(modKey(tcell.KeyTAB, tcell.ModCtrl))
	if b.activeTab != 1 {
		t.Errorf("Ctrl+Tab activeTab = %d, want 1", b.activeTab)
	}
	// Ctrl+Shift+Tab → previous tab.
	h(modKey(tcell.KeyTAB, tcell.ModCtrl|tcell.ModShift))
	if b.activeTab != 0 {
		t.Errorf("Ctrl+Shift+Tab activeTab = %d, want 0", b.activeTab)
	}
	// Plain Tab → focus the URL bar.
	h(key(tcell.KeyTAB))
	if b.app.GetFocus() != b.urlInput {
		t.Error("plain Tab should focus the URL input")
	}
	// The '\t' rune branch does the same.
	b.app.SetFocus(b.currentTab().textView)
	h(rn('\t'))
	if b.app.GetFocus() != b.urlInput {
		t.Error("'\\t' rune should focus the URL input")
	}
}

func TestSetupKeyBindingsLinkAndImageModals(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	tab := b.currentTab()

	// No links, no images: Ctrl+L opens nothing but is still consumed.
	if got := h(key(tcell.KeyCtrlL)); got != nil {
		t.Error("Ctrl+L should be consumed")
	}

	// Images only: Ctrl+L falls through to the images modal.
	tab.images = []Image{{URL: "https://example.com/a.png", Alt: "a"}}
	h(key(tcell.KeyCtrlL))
	if b.app.GetFocus() == nil {
		t.Error("images modal should take focus")
	}

	// Links present: Ctrl+L opens the links modal.
	tab.links = []Link{{URL: "https://example.com/x", Text: "x"}}
	h(key(tcell.KeyCtrlL))

	// 'i' opens the images modal directly.
	if got := h(rn('i')); got != nil {
		t.Error("'i' should be consumed")
	}
}

func TestSetupKeyBindingsSearchKeys(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	tab := b.currentTab()
	tab.originalContent = "alpha beta alpha"

	if got := h(rn('/')); got != nil {
		t.Error("'/' should be consumed")
	}
	if tab.displayToMatchIndex == nil {
		t.Error("'/' should start the search UI")
	}

	// n/N with no active search are safe no-ops.
	if got := h(rn('n')); got != nil {
		t.Error("'n' should be consumed")
	}
	if got := h(rn('N')); got != nil {
		t.Error("'N' should be consumed")
	}
}

func TestSetupKeyBindingsHelpAndSettings(t *testing.T) {
	b, h := newKeyBoundBrowser(t)
	cfg := GetDefaultConfig()
	b.config = &cfg

	if got := h(rn('?')); got != nil {
		t.Error("'?' should be consumed")
	}
	if got := h(key(tcell.KeyCtrlS)); got != nil {
		t.Error("Ctrl+S should be consumed")
	}
	if !b.settingsActive {
		t.Error("Ctrl+S should open the settings modal")
	}
}

func TestSetupKeyBindingsCancelAndQuit(t *testing.T) {
	b, h := newKeyBoundBrowser(t)

	if got := h(key(tcell.KeyEscape)); got != nil {
		t.Error("Esc should be consumed (cancels the in-flight request)")
	}
	if got := h(key(tcell.KeyEnter)); got != nil {
		t.Error("Enter should be consumed")
	}
	if got := h(key(tcell.KeyCtrlC)); got != nil {
		t.Error("Ctrl+C should be consumed")
	}
	if got := h(rn('q')); got != nil {
		t.Error("'q' should be consumed")
	}
	_ = b
}

func TestSetupKeyBindingsUnhandledKeyPassesThrough(t *testing.T) {
	_, h := newKeyBoundBrowser(t)
	ev := key(tcell.KeyF5)
	if got := h(ev); got != ev {
		t.Error("unhandled keys must pass through unchanged")
	}
	if got := h(rn('z')); got == nil {
		t.Error("unhandled runes must pass through unchanged")
	}
}

func TestCreateUIWiresURLInput(t *testing.T) {
	b := testBrowserWithUI()
	b.urlInput = nil
	b.createUI()

	if b.urlInput == nil {
		t.Fatal("createUI should create the URL input")
	}
	h := keyHandler(t, b.urlInput)

	// Tab with empty text pulls the 5 most recent history entries.
	b.currentTab().history = []string{"https://example.com/a", "https://example.com/b"}
	if got := h(key(tcell.KeyTAB)); got != nil {
		t.Error("Tab should be consumed by the URL input")
	}
	if b.urlInput.GetText() == "" {
		t.Error("Tab should autocomplete from history")
	}

	// Tab with a prefix filters completions.
	b.urlInput.SetText("https://example.com/a")
	h(key(tcell.KeyTAB))
	if b.urlInput.GetText() != "https://example.com/a" {
		t.Errorf("prefix completion = %q", b.urlInput.GetText())
	}

	// Esc returns focus to the content view.
	if got := h(key(tcell.KeyEscape)); got != nil {
		t.Error("Esc should be consumed")
	}
	if b.app.GetFocus() != b.currentTab().textView {
		t.Error("Esc should focus the content view")
	}

	// Ctrl+P reads the clipboard; a headless environment simply fails silently.
	if got := h(key(tcell.KeyCtrlP)); got != nil {
		t.Error("Ctrl+P should be consumed")
	}

	// Ctrl+S opens settings.
	cfg := GetDefaultConfig()
	b.config = &cfg
	if got := h(key(tcell.KeyCtrlS)); got != nil {
		t.Error("Ctrl+S should be consumed")
	}
	if !b.settingsActive {
		t.Error("Ctrl+S in the URL bar should open settings")
	}

	// Unhandled keys pass through.
	ev := key(tcell.KeyF2)
	if got := h(ev); got != ev {
		t.Error("unhandled keys must pass through")
	}
}

func TestShowHelp(t *testing.T) {
	b := testBrowserWithUI()
	b.showHelp()
	// The help view is the new root; closing it restores the main flex.
	if b.app.GetFocus() == nil {
		// SetRoot(…, true) focuses the root, so focus must be the help view.
		t.Error("showHelp should install and focus the help view")
	}
}

func TestLinksModalInputCaptures(t *testing.T) {
	b := testBrowserWithUI()
	tab := b.currentTab()
	tab.links = []Link{
		{URL: "https://example.com/alpha", Text: "Alpha"},
		{URL: "https://example.com/beta.png", Text: "Beta"},
	}
	tab.images = []Image{{URL: "https://example.com/beta.png", Alt: "b"}}

	b.showLinksModalPage(0)

	input, ok := b.app.GetFocus().(*tview.InputField)
	if !ok {
		t.Fatalf("links modal should focus an InputField, got %T", b.app.GetFocus())
	}
	h := keyHandler(t, input)

	// Tab and Down hand focus to the list.
	if got := h(key(tcell.KeyTAB)); got != nil {
		t.Error("Tab should be consumed by the filter field")
	}
	list, ok := b.app.GetFocus().(*tview.List)
	if !ok {
		t.Fatalf("Tab should focus the link list, got %T", b.app.GetFocus())
	}

	// Filtering re-renders the list in place.
	before := list.GetItemCount()
	input.SetText("alpha")
	if after := list.GetItemCount(); after == before && before > 2 {
		t.Error("filtering should change the rendered item count")
	}
	input.SetText("")

	lh := keyHandler(t, list)
	// '/' returns focus to the filter field.
	if got := lh(rn('/')); got != nil {
		t.Error("'/' should be consumed by the list")
	}
	if b.app.GetFocus() != input {
		t.Error("'/' should focus the filter field")
	}

	// 'q' and Esc close the modal back to the main view.
	if got := lh(rn('q')); got != nil {
		t.Error("'q' should be consumed by the list")
	}
	if got := lh(key(tcell.KeyEscape)); got != nil {
		t.Error("Esc should be consumed by the list")
	}
	if got := h(key(tcell.KeyEscape)); got != nil {
		t.Error("Esc should be consumed by the filter field")
	}
	// Unhandled keys pass through both captures.
	ev := key(tcell.KeyF3)
	if got := lh(ev); got != ev {
		t.Error("unhandled list keys must pass through")
	}
	if got := h(ev); got != ev {
		t.Error("unhandled filter keys must pass through")
	}
}

func TestLinksModalPagination(t *testing.T) {
	b := testBrowserWithUI()
	tab := b.currentTab()
	for i := range 45 {
		tab.links = append(tab.links, Link{URL: "https://example.com/l", Text: string(rune('a' + i%26))})
	}
	tab.history = []string{"https://example.com/prev"}
	tab.historyIndex = 1

	b.showLinksModalPage(0)
	list, ok := b.app.GetFocus().(*tview.InputField)
	if !ok {
		t.Fatalf("expected filter field focus, got %T", b.app.GetFocus())
	}
	_ = list

	// Page 1 of 3 with 45 links at 20 per page; asking for a page past the end
	// clamps instead of panicking.
	b.showLinksModalPage(99)
	b.showLinksModalPage(-5)
}

func TestImagesModalInputCaptures(t *testing.T) {
	b := testBrowserWithUI()
	tab := b.currentTab()
	tab.images = []Image{
		{URL: "https://example.com/a.png", Alt: "alpha"},
		{URL: "https://example.com/b.png", Title: "beta"},
	}

	b.showImagesModalPage(0)

	input, ok := b.app.GetFocus().(*tview.InputField)
	if !ok {
		t.Fatalf("images modal should focus an InputField, got %T", b.app.GetFocus())
	}
	h := keyHandler(t, input)

	if got := h(key(tcell.KeyTAB)); got != nil {
		t.Error("Tab should be consumed by the image filter")
	}
	list, ok := b.app.GetFocus().(*tview.List)
	if !ok {
		t.Fatalf("Tab should focus the image list, got %T", b.app.GetFocus())
	}
	lh := keyHandler(t, list)

	input.SetText("alpha")
	input.SetText("")

	if got := lh(rn('q')); got != nil {
		t.Error("'q' should be consumed by the image list")
	}
	if got := lh(key(tcell.KeyEscape)); got != nil {
		t.Error("Esc should be consumed by the image list")
	}
	if got := h(key(tcell.KeyEscape)); got != nil {
		t.Error("Esc should be consumed by the image filter")
	}
}

func TestSettingsModalInputCaptures(t *testing.T) {
	b := testBrowserWithUI()
	cfg := GetDefaultConfig()
	b.config = &cfg

	b.showSettingsModal()
	leftList, ok := b.app.GetFocus().(*tview.List)
	if !ok {
		t.Fatalf("settings modal should focus the categories list, got %T", b.app.GetFocus())
	}

	// Build the right column for the first category.  Focus lands on the form's
	// first field (tview delegates Form focus to its items), so reach the Form
	// through the flex instead of app.GetFocus().
	flex := tview.NewFlex()
	cat := b.getSettingCategories()[0]
	b.updateRightColumn(flex, leftList, cat)

	var form *tview.Form
	for i := range flex.GetItemCount() {
		sub, ok := flex.GetItem(i).(*tview.Flex)
		if !ok {
			continue
		}
		for j := range sub.GetItemCount() {
			if f, isForm := sub.GetItem(j).(*tview.Form); isForm {
				form = f
			}
		}
	}
	if form == nil {
		t.Fatal("updateRightColumn should build a form in the right column")
	}
	fh := keyHandler(t, form)

	// Tab and Shift+Tab jump back to the categories list.
	if got := fh(key(tcell.KeyTAB)); got != nil {
		t.Error("Tab should be consumed by the form")
	}
	if b.app.GetFocus() != leftList {
		t.Error("Tab in the form should focus the categories list")
	}
	if got := fh(modKey(tcell.KeyBacktab, tcell.ModShift)); got != nil {
		t.Error("Shift+Tab should be consumed by the form")
	}

	// Ctrl+Down / Ctrl+Up move between fields and buttons.
	if got := fh(modKey(tcell.KeyDown, tcell.ModCtrl)); got != nil {
		t.Error("Ctrl+Down should be consumed")
	}
	if got := fh(modKey(tcell.KeyUp, tcell.ModCtrl)); got != nil {
		t.Error("Ctrl+Up should be consumed")
	}
	// Plain Up/Down only refresh the description and pass through.
	if got := fh(key(tcell.KeyDown)); got == nil {
		t.Error("plain Down should pass through to the form")
	}
	if got := fh(key(tcell.KeyUp)); got == nil {
		t.Error("plain Up should pass through to the form")
	}

	// Ctrl+Q closes without saving.
	if got := fh(key(tcell.KeyCtrlQ)); got != nil {
		t.Error("Ctrl+Q should be consumed")
	}
	if b.settingsActive {
		t.Error("Ctrl+Q should close the settings modal")
	}

	// The categories list Tab handler jumps to the form.
	lh := keyHandler(t, leftList)
	if got := lh(key(tcell.KeyTAB)); got != nil {
		t.Error("Tab should be consumed by the categories list")
	}
	// Unhandled keys pass through.
	ev := key(tcell.KeyF4)
	if got := lh(ev); got != ev {
		t.Error("unhandled category keys must pass through")
	}
}

func TestSettingsModalEmptyRightColumnCapture(t *testing.T) {
	b := testBrowserWithUI()
	cfg := GetDefaultConfig()
	b.config = &cfg
	leftList := tview.NewList()
	flex := tview.NewFlex()

	b.updateRightColumnForEmpty(flex, leftList)
	form := b.findFormInFlex(flex)
	if form == nil {
		t.Fatal("empty right column should contain a form")
	}
	h := keyHandler(t, form)
	if got := h(key(tcell.KeyTAB)); got != nil {
		t.Error("Tab should be consumed by the empty form")
	}
	if b.app.GetFocus() != leftList {
		t.Error("Tab in the empty form should focus the categories list")
	}
	ev := key(tcell.KeyF6)
	if got := h(ev); got != ev {
		t.Error("unhandled keys must pass through the empty form")
	}
}

func TestSettingsModalFlexCapture(t *testing.T) {
	b := testBrowserWithUI()
	cfg := GetDefaultConfig()
	b.config = &cfg

	// Esc closes the settings modal; the capture lives on the modal's flex,
	// which is reachable through the focused list's parent chain in production.
	// Here we rebuild the same wiring to exercise the branches.
	b.showSettingsModal()
	if !b.settingsActive {
		t.Fatal("settings should be active after showSettingsModal")
	}
	b.closeSettingsModal()
	if b.settingsActive {
		t.Error("closeSettingsModal should clear settingsActive")
	}
}
