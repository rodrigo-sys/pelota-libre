package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func playMPV(url string) error {
	cmd := exec.Command("mpv", url)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveURL(source Source, url string) string {
	resolved, err := source.Resolve(url)
	if err != nil || resolved == "" {
		return url
	}
	return resolved
}

func stringsContains(s, sub string) bool {
	return strings.Contains(s, sub)
}
