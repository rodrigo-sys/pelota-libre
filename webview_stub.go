//go:build !cgo

package main

import "fmt"

func openWebView(url string) {
	fmt.Println("WebView not available (CGo disabled), opening in browser instead")
	openBrowser(url)
}
