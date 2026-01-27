package utils

import (
	"log"
	"net/url"
	"strings"

	"github.com/valyala/fasthttp"
)

func ExtractChain(path string) (chain string, extra string, ok bool) {
	const prefix = "/chain/"

	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}

	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return "", "", false
	}

	// Split once: "<chain>" or "<chain>/extra/..."
	parts := strings.SplitN(rest, "/", 2)

	chain = parts[0]
	if chain == "" {
		return "", "", false
	}

	if len(parts) == 2 {
		extra = "/" + parts[1] // keep leading slash for routing
	}

	return chain, extra, true
}

func ClientIPFromXFF(ctx *fasthttp.RequestCtx) string {
	xff := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if xff != "" {
		// "client, proxy1, proxy2" -> "client"
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		return strings.TrimSpace(xff)
	}
	return ctx.RemoteIP().String()
}

// ExtractAPIKeyAndPath extracts API key and path from request context
func ExtractAPIKeyAndPath(ctx *fasthttp.RequestCtx) (string, string, error) {
	uri := string(ctx.RequestURI())
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	path := parsedURI.Path
	apiKey := extractAPIKey(path)

	// If API key is not found in path, check headers
	if apiKey == "" {
		apiKey = string(ctx.Request.Header.Peek("X-API-Key"))
	}

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
