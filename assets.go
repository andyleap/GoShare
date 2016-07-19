package main

import "net/http"

//go:generate -command asset go run asset.go
//go:generate asset SimpleGrid.css

func css(a asset) http.Handler {
	return a
}
