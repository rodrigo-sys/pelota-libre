package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var cacheTTL = 5 * time.Minute

func main() {
	webFlag := flag.Bool("web", false, "Use browser instead of webview")
	refreshFlag := flag.Bool("refresh", false, "Force refresh cache")
	sourceName := flag.String("source", "pelotalibre", "Source provider")
	flag.Parse()

	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "--source=") {
			*sourceName = strings.TrimPrefix(arg, "--source=")
		}
		if arg == "--source" && i+1 < len(os.Args) {
			*sourceName = os.Args[i+1]
		}
	}

	source, err := lookupSource(*sourceName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(source, *sourceName, *webFlag, *refreshFlag),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getEvents(source Source, force bool) ([]Event, error) {
	name := source.Name()
	if !force && cacheFresh(eventsCachePath(name), cacheTTL) {
		events, err := loadEvents(name)
		if err == nil && len(events) > 0 {
			return events, nil
		}
	}
	events, err := source.FetchEvents()
	if err != nil {
		return nil, err
	}
	if err := saveEvents(events, name); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cache write failed: %v\n", err)
	}
	return events, nil
}


