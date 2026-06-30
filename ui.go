package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateLoading state = iota
	stateMatchList
	stateStreamList
	stateSourceMenu
	statePlaying
)

type (
	eventsLoadedMsg    []Event
	errMsg             error
	playbackDoneMsg    struct{}
	healthUpdateMsg    []streamItem
)

type playbackMode int

const (
	modeWebView playbackMode = iota
	modeBrowser
)

var sourceNames = []string{"la18hd", "pelotalibre", "pirlotv"}

var healthClient = &http.Client{Timeout: 3 * time.Second}

// ----- Items -----

type matchItem struct {
	title    string
	time     string
	league   string
	date     string
	channels []string
}

func (i matchItem) Title() string       { return i.title }
func (i matchItem) Description() string { return "" }
func (i matchItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s", i.time, i.league, i.title, i.date)
}

type streamItem struct {
	lang       string
	channel    string
	link       string
	statusCode int // -1 = unknown, 0 = error, otherwise HTTP status
}

func (i streamItem) Title() string {
	s := fmt.Sprintf("%s │ %s", i.lang, i.channel)
	if i.isOffline() {
		s += " [off]"
	}
	return s
}
func (i streamItem) Description() string { return i.link }
func (i streamItem) FilterValue() string { return i.lang + " " + i.channel }
func (i streamItem) isOffline() bool {
	return i.statusCode == 0 || i.statusCode >= 400
}

type sourceItem struct {
	name    string
	current bool
}

func (i sourceItem) Title() string {
	if i.current {
		return fmt.Sprintf("> %s <", i.name)
	}
	return i.name
}
func (i sourceItem) Description() string { return "" }
func (i sourceItem) FilterValue() string { return i.name }

// ----- Delegates -----

type matchDelegate struct{}

func (d matchDelegate) Height() int                               { return 1 }
func (d matchDelegate) Spacing() int                              { return 0 }
func (d matchDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd   { return nil }
func (d matchDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(matchItem)
	if !ok {
		return
	}
	style := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(1)
	if index == m.Index() {
		style = style.Bold(true).
			Foreground(lipgloss.Color("63")).
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("63")).
			PaddingLeft(1)
	}
	fmt.Fprint(w, style.Render(fmt.Sprintf("%-6s %-22s %s", it.time, it.league, it.title)))
}

type streamDelegate struct{}

func (d streamDelegate) Height() int                               { return 2 }
func (d streamDelegate) Spacing() int                              { return 0 }
func (d streamDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd   { return nil }
func (d streamDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(streamItem)
	if !ok {
		return
	}
	sel := index == m.Index()

	mainStyle := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(1)
	if sel {
		mainStyle = mainStyle.Bold(true).
			Foreground(lipgloss.Color("63")).
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("63")).
			PaddingLeft(1)
	}
	mainText := fmt.Sprintf("%s │ %s", it.lang, it.channel)
	if it.isOffline() {
		off := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		mainText += " " + off.Render("[off]")
	}

	linkStyle := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(1).Foreground(lipgloss.Color("8"))
	if sel {
		linkStyle = linkStyle.Bold(true).
			Foreground(lipgloss.Color("63")).
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("63")).
			PaddingLeft(1)
	}
	if it.isOffline() {
		linkStyle = linkStyle.Foreground(lipgloss.Color("9"))
	}

	fmt.Fprint(w, mainStyle.Render(mainText))
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, linkStyle.Render(it.link))
}

type sourceDelegate struct{}

func (d sourceDelegate) Height() int                               { return 1 }
func (d sourceDelegate) Spacing() int                              { return 0 }
func (d sourceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd   { return nil }
func (d sourceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(sourceItem)
	if !ok {
		return
	}
	style := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(1)
	if index == m.Index() {
		style = style.Bold(true).
			Foreground(lipgloss.Color("63")).
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("63")).
			PaddingLeft(1)
	}
	fmt.Fprint(w, style.Render(it.Title()))
}

// ----- Model -----

type model struct {
	state             state
	source            Source
	sourceName        string
	mode              playbackMode
	refresh           bool
	events            []Event
	matchList         list.Model
	matchItems        []matchItem
	streamList        list.Model
	streamItems       []streamItem
	currentMatchTitle string
	sourceMenu        list.Model
	spinner           spinner.Model
	err               error
	width             int
	height            int
	ready             bool
	prevState         state
}

func initialModel(source Source, name string, web bool, refresh bool) model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	s.Spinner = spinner.Dot

	mode := modeWebView
	if web {
		mode = modeBrowser
	}

	m := model{
		state:      stateLoading,
		source:     source,
		sourceName: name,
		mode:       mode,
		refresh:    refresh,
		spinner:    s,
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchEventsCmd(m.source, m.refresh))
}

// ----- Commands -----

func fetchEventsCmd(source Source, force bool) tea.Cmd {
	return func() tea.Msg {
		events, err := getEvents(source, force)
		if err != nil {
			return errMsg(err)
		}
		return eventsLoadedMsg(events)
	}
}

func playMPVCmd(url string) tea.Cmd {
	return func() tea.Msg {
		playMPV(url)
		return playbackDoneMsg{}
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		openBrowser(url)
		return playbackDoneMsg{}
	}
}

func openWebViewCmd(url string) tea.Cmd {
	return func() tea.Msg {
		openWebView(url)
		return playbackDoneMsg{}
	}
}

func healthCheckCmd(items []streamItem) tea.Cmd {
	return func() tea.Msg {
		updated := make([]streamItem, len(items))
		copy(updated, items)
		var wg sync.WaitGroup
		for i := range updated {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				code, err := checkHealth(updated[idx].link)
				if err != nil {
					updated[idx].statusCode = 0
				} else {
					updated[idx].statusCode = code
				}
			}(i)
		}
		wg.Wait()
		return healthUpdateMsg(updated)
	}
}

func checkHealth(url string) (int, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := healthClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// ----- Builders -----

func buildMatchItems(events []Event) []matchItem {
	seen := map[string]bool{}
	var items []matchItem
	for _, e := range events {
		if seen[e.Title] {
			continue
		}
		seen[e.Title] = true
		var chans []string
		for _, ev := range events {
			if ev.Title == e.Title {
				c := prettyChannel(channelFromLink(ev.Link))
				if c != "" {
					chans = append(chans, c)
				}
			}
		}
		items = append(items, matchItem{
			title:    e.Title,
			time:     e.Time,
			league:   e.League,
			date:     e.Date,
			channels: chans,
		})
	}
	return items
}

func buildMatchList(items []matchItem, sourceName string, w, h int) list.Model {
	var listItems []list.Item
	for _, it := range items {
		listItems = append(listItems, it)
	}
	l := list.New(listItems, matchDelegate{}, w, h)
	l.Title = fmt.Sprintf("Matches [%s]", sourceName)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("63")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 1)
	l.Styles.PaginationStyle = lipgloss.NewStyle().Padding(0, 1)
	l.Styles.HelpStyle = lipgloss.NewStyle().Padding(0, 1)
	l.KeyMap.Quit.SetKeys("q")
	l.KeyMap.ForceQuit.SetKeys("ctrl+c")
	return l
}

func buildStreamListFromEvents(events []Event, matchTitle string, w, h int) (list.Model, []streamItem) {
	var items []streamItem
	seen := map[string]bool{}
	for _, e := range events {
		if e.Title != matchTitle || seen[e.Link] {
			continue
		}
		seen[e.Link] = true
		items = append(items, streamItem{
			lang:       displayLang(e.Language),
			channel:    prettyChannel(channelFromLink(e.Link)),
			link:       e.Link,
			statusCode: -1,
		})
	}
	return buildStreamList(items, matchTitle, w, h), items
}

func buildStreamList(items []streamItem, matchTitle string, w, h int) list.Model {
	var listItems []list.Item
	for _, it := range items {
		listItems = append(listItems, it)
	}
	l := list.New(listItems, streamDelegate{}, w, h)
	l.Title = fmt.Sprintf("Streams — %s", matchTitle)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("63")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 1)
	l.Styles.PaginationStyle = lipgloss.NewStyle().Padding(0, 1)
	l.Styles.HelpStyle = lipgloss.NewStyle().Padding(0, 1)
	l.KeyMap.Quit.SetKeys("q")
	l.KeyMap.ForceQuit.SetKeys("ctrl+c")
	return l
}

func buildSourceMenu(current string, w, h int) list.Model {
	var items []list.Item
	for _, name := range sourceNames {
		items = append(items, sourceItem{name: name, current: name == current})
	}
	l := list.New(items, sourceDelegate{}, w, h)
	l.Title = "Source"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("63")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 1)
	l.KeyMap.Quit.SetKeys("q")
	l.KeyMap.ForceQuit.SetKeys("ctrl+c")
	return l
}

// ----- Update -----

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.ready = true
		}
		switch m.state {
		case stateMatchList:
			if m.matchList.Height() > 0 {
				m.matchList.SetSize(msg.Width, msg.Height-4)
			}
		case stateStreamList:
			if m.streamList.Height() > 0 {
				m.streamList.SetSize(msg.Width, msg.Height-4)
			}
		case stateSourceMenu:
			if m.sourceMenu.Height() > 0 {
				m.sourceMenu.SetSize(msg.Width, msg.Height-4)
			}
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateMatchList:
			return m.matchListUpdate(msg)
		case stateStreamList:
			return m.streamListUpdate(msg)
		case stateSourceMenu:
			return m.sourceMenuUpdate(msg)
		case stateLoading:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		case statePlaying:
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}

	case eventsLoadedMsg:
		m.events = []Event(msg)
		m.matchItems = buildMatchItems(m.events)
		m.matchList = buildMatchList(m.matchItems, m.sourceName, m.width, m.height-4)
		m.state = stateMatchList
		return m, nil

	case errMsg:
		m.err = msg
		m.state = stateMatchList
		return m, nil

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case playbackDoneMsg:
		m.state = stateStreamList
		return m, nil

	case healthUpdateMsg:
		items := []streamItem(msg)
		m.streamItems = items
		m.streamList = buildStreamList(items, m.currentMatchTitle, m.width, m.height-4)
		return m, nil
	}

	return m, nil
}

func (m model) matchListUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "h", "esc":
		return m, tea.Quit
	case "l", "enter":
		it := m.matchList.SelectedItem()
		if it == nil {
			return m, nil
		}
		mi, ok := it.(matchItem)
		if !ok || mi.title == "" {
			return m, nil
		}
		m.currentMatchTitle = mi.title
		list, items := buildStreamListFromEvents(m.events, mi.title, m.width, m.height-4)
		m.streamList = list
		m.streamItems = items
		m.state = stateStreamList
		return m, healthCheckCmd(items)
	case "s":
		m.prevState = m.state
		m.sourceMenu = buildSourceMenu(m.sourceName, m.width, m.height-4)
		m.state = stateSourceMenu
		return m, nil
	case "r":
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchEventsCmd(m.source, true))
	}
	var cmd tea.Cmd
	m.matchList, cmd = m.matchList.Update(msg)
	return m, cmd
}

func (m model) streamListUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "h", "esc":
		m.state = stateMatchList
		return m, nil
	case "l", "enter":
		it := m.streamList.SelectedItem()
		if it == nil {
			return m, nil
		}
		si, ok := it.(streamItem)
		if !ok {
			return m, nil
		}
		url := resolveURL(m.source, si.link)
		m.state = statePlaying
		switch m.mode {
		case modeBrowser:
			return m, openBrowserCmd(url)
		default:
			return m, openWebViewCmd(url)
		}
	case "b":
		it := m.streamList.SelectedItem()
		if it == nil {
			return m, nil
		}
		si, ok := it.(streamItem)
		if !ok {
			return m, nil
		}
		m.state = statePlaying
		return m, openBrowserCmd(resolveURL(m.source, si.link))
	case "w":
		if m.mode == modeWebView {
			m.mode = modeBrowser
		} else {
			m.mode = modeWebView
		}
		return m, nil
	case "s":
		m.prevState = m.state
		m.sourceMenu = buildSourceMenu(m.sourceName, m.width, m.height-4)
		m.state = stateSourceMenu
		return m, nil
	}
	var cmd tea.Cmd
	m.streamList, cmd = m.streamList.Update(msg)
	return m, cmd
}

func (m model) sourceMenuUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "h", "esc":
		m.state = m.prevState
		return m, nil
	case "l", "enter":
		it := m.sourceMenu.SelectedItem()
		if it == nil {
			return m, nil
		}
		si, ok := it.(sourceItem)
		if !ok {
			return m, nil
		}
		if si.name == m.sourceName {
			m.state = m.prevState
			return m, nil
		}
		s, err := lookupSource(si.name)
		if err != nil {
			return m, nil
		}
		m.source = s
		m.sourceName = si.name
		m.state = stateLoading
		return m, tea.Batch(m.spinner.Tick, fetchEventsCmd(m.source, true))
	}
	var cmd tea.Cmd
	m.sourceMenu, cmd = m.sourceMenu.Update(msg)
	return m, cmd
}

// ----- View -----

func (m model) View() string {
	if !m.ready {
		return ""
	}

	switch m.state {
	case stateLoading:
		return m.loadingView()
	case stateMatchList:
		return m.matchListView()
	case stateStreamList:
		return m.streamListView()
	case stateSourceMenu:
		return m.sourceMenuView()
	case statePlaying:
		return m.playingView()
	default:
		return ""
	}
}

func (m model) loadingView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)
	return style.Render(fmt.Sprintf("%s Loading events [%s]...", m.spinner.View(), m.sourceName))
}

func (m model) playingView() string {
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)
	return style.Render("Playing... (close player to return)")
}

func (m model) matchListView() string {
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(0, 1)
		return fmt.Sprintf("Error: %s\nr: retry  s: source  h: quit", errStyle.Render(m.err.Error()))
	}

	if m.matchList.Height() == 0 {
		m.matchList.SetSize(m.width, m.height-4)
	}

	preview := ""
	if it := m.matchList.SelectedItem(); it != nil {
		if mi, ok := it.(matchItem); ok {
			chans := strings.Join(mi.channels, ", ")
			style := lipgloss.NewStyle().
				Width(m.width - 2).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63"))
			preview = style.Render(fmt.Sprintf("League: %s  |  Time: %s  |  %s\nChannels: %s",
				mi.league, mi.time, mi.date, chans))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top,
		m.matchList.View(),
		preview,
	)
}

func (m model) streamListView() string {
	if m.streamList.Height() == 0 {
		m.streamList.SetSize(m.width, m.height-4)
	}

	modeLabel := "webview"
	if m.mode == modeBrowser {
		modeLabel = "browser"
	}
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Padding(0, 1)
	help := helpStyle.Render(fmt.Sprintf("l: play (%s)  |  w: switch  |  b: force browser  |  h: back  |  s: source  |  q: quit", modeLabel))

	return lipgloss.JoinVertical(lipgloss.Top,
		m.streamList.View(),
		help,
	)
}

func (m model) sourceMenuView() string {
	if m.sourceMenu.Height() == 0 {
		m.sourceMenu.SetSize(m.width, m.height-4)
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Padding(0, 1)
	help := helpStyle.Render("l: select  |  h: back  |  q: quit")

	return lipgloss.JoinVertical(lipgloss.Top,
		m.sourceMenu.View(),
		help,
	)
}
