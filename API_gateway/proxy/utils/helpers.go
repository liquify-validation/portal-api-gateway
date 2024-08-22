package utils

import (
	"fmt"
	"log"
	"strings"

	"github.com/valyala/fasthttp"
)

// ExtractAPIKeyAndPath extracts API key and path from request context
func ExtractAPIKeyAndPath(ctx *fasthttp.RequestCtx) (string, string, error) {
	apiKey := string(ctx.Request.Header.Peek("x-api-key"))
	path := string(ctx.Path())
	if apiKey == "" || path == "" {
		return "", "", fmt.Errorf("API key or path is missing")
	}
	return apiKey, path, nil
}

// ExtractAdditionalPath extracts additional path information
func ExtractAdditionalPath(path string) string {
	return strings.TrimPrefix(path, "/")
}

// isWebSocketRequest checks if the request is a WebSocket upgrade request
func IsWebSocketRequest(ctx *fasthttp.RequestCtx) bool {
	return string(ctx.Request.Header.Peek("Upgrade")) == "websocket"
}

// HandleProxyError handles errors during proxy requests
func HandleProxyError(ctx *fasthttp.RequestCtx, err error) {
	log.Printf("Error proxying request: %s", err)
	ctx.Error("Error proxying request", fasthttp.StatusInternalServerError)
}
