package utils

import (
	"github.com/valyala/fasthttp"
	"log"
	"net/url"
	"strings"
)

// ExtractAPIKeyAndPath extracts API key and path from request context
func ExtractAPIKeyAndPath(ctx *fasthttp.RequestCtx) (string, string, error) {
	uri := string(ctx.RequestURI())
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	path := parsedURI.Path
	apiKey := extractAPIKey(path)
	return apiKey, path, nil
}

// extractAPIKey extracts API key from the query string
func extractAPIKey(queryString string) string {
	parts := strings.Split(queryString, "=")
	if len(parts) < 2 || parts[0] != "/api" {
		return ""
	}

	keys := strings.Split(parts[1], "/")
	return keys[0]
}

// ExtractAdditionalPath extracts additional path information
func ExtractAdditionalPath(path string, queryString string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		// parts[0] will be an empty string because the string starts with '/'
		remainingParts := parts[2:]
		reconstructedPath := "/" + strings.Join(remainingParts, "/")

		if queryString != "" {
			decodedQueryString, err := url.QueryUnescape(queryString)
			if err != nil {
				return ""
			}
			reconstructedPath = reconstructedPath + "?" + decodedQueryString
		}

		return reconstructedPath
	} else {
		return ""
	}
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
