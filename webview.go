package main

import (
	"fmt"
	"os"

	"github.com/webview/webview_go"
)

func openWebView(url string) {
	debug := os.Getenv("PELOTA_DEBUG") != ""
	w := webview.New(debug)
	if w == nil {
		fmt.Fprintln(os.Stderr, "webview: failed to create window, falling back to browser")
		openBrowser(url)
		return
	}
	defer w.Destroy()

	w.SetTitle("pelota")
	w.SetSize(480, 320, webview.HintNone)
	w.Navigate(url)
	w.Run()
}
