package proxy

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"proxy/metrics"
	"proxy/utils"
)

var client = &fasthttp.Client{
	MaxConnsPerHost: 10000,
	ReadTimeout:     30 * time.Second,
	WriteTimeout:    30 * time.Second,
}

// Custom error type that includes the HTTP status code
type ProxyError struct {
	Msg    string
	Status int
}

func (e *ProxyError) Error() string { return e.Msg }

// ProxyHttpRequest proxies an incoming request to one of the upstreams in chainMap[chain]
func ProxyHttpRequest(ctx *fasthttp.RequestCtx, req *fasthttp.Request, chain string, chainMap map[string][]string, apiKey string, keyData map[string]interface{}) {
	queryString := string(ctx.QueryArgs().QueryString())
	path := utils.ExtractAdditionalPath(string(ctx.Path()), queryString)

	// Propagate X-Forwarded-For (don’t overwrite if already set on the outgoing req)
	if req.Header.Peek("X-Forwarded-For") == nil {
		if xff := ctx.Request.Header.Peek("X-Forwarded-For"); len(xff) > 0 {
			req.Header.Set("X-Forwarded-For", string(xff))
		}
	}

	// Inject API key once if not present
	if req.Header.Peek("API-Key") == nil {
		req.Header.Set("API-Key", apiKey)
	}

	// SSE passthrough (prefer Accept header; path check kept for backward compat)
	acceptHeader := string(ctx.Request.Header.Peek("Accept"))
	isSSE := strings.Contains(acceptHeader, "text/event-stream") || strings.Contains(path, "stream")

	//workaround as thor has streaming keywork in it's queries
	isTHOR := strings.Contains(strings.ToLower(chain), strings.ToLower("thor"))
	
	if isSSE && !isTHOR{
		proxySSE(chainMap[chain][0]+path, ctx, chain, apiKey, keyData)
		return
	}

	maxRetries := 3
	responseChan := make(chan *fasthttp.Response, 1)
	errChan := make(chan error, 1)

	// Cancellable context for the worker goroutine
	proxyCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer close(responseChan)
		defer close(errChan)

		chainCode, ok := chainMap[chain]
		if !ok || len(chainCode) == 0 {
			errChan <- &ProxyError{Msg: "failed to proxy request: invalid chain configuration", Status: fasthttp.StatusBadRequest}
			return
		}

		var lastErr error

		for attempt := 0; attempt < maxRetries; attempt++ {
			select {
			case <-proxyCtx.Done():
				errChan <- &ProxyError{Msg: "request cancelled", Status: fasthttp.StatusRequestTimeout}
				return
			default:
			}

			uri := chainCode[attempt%len(chainCode)] + path
			req.SetRequestURI(uri)

			backendResp := fasthttp.AcquireResponse()
			if err := client.Do(req, backendResp); err != nil {
				// Transport error → retry
				log.Printf("proxy network error: %s -> %v", uri, err)
				fasthttp.ReleaseResponse(backendResp)
				lastErr = &ProxyError{Msg: err.Error(), Status: fasthttp.StatusBadGateway}
				continue
			}

			status := backendResp.StatusCode()

			// Treat any 2xx as success
			if status >= 200 && status < 300 {
				responseChan <- backendResp
				return
			}

			// Optional: retry on upstream 5xx
			if status >= 500 && status <= 599 && attempt < maxRetries-1 {
				log.Printf("proxy upstream %d from %s; retrying...", status, uri)
				fasthttp.ReleaseResponse(backendResp)
				continue
			}

			// Hand non-2xx back so caller can forward status + body to client
			responseChan <- backendResp
			return
		}

		if lastErr != nil {
			errChan <- lastErr
		} else {
			errChan <- &ProxyError{Msg: "proxy failed", Status: fasthttp.StatusBadGateway}
		}
	}()

	select {
	case backendResp := <-responseChan:
		if backendResp == nil {
			ctx.Error("Error proxying request: no response received", fasthttp.StatusBadGateway)
			metrics.RequestsTotal.WithLabelValues("502").Inc()
			return
		}

		// Forward upstream status and body as-is (even if non-2xx)
		ctx.SetStatusCode(backendResp.StatusCode())

		// Copy all headers except hop-by-hop ones. This preserves Content-Encoding/Vary/etc.
		hopByHop := map[string]struct{}{
			"connection":            {},
			"keep-alive":            {},
			"proxy-authenticate":    {},
			"proxy-authorization":   {},
			"te":                    {},
			"trailer":               {},
			"transfer-encoding":     {},
			"upgrade":               {},
		}

		backendResp.Header.VisitAll(func(k, v []byte) {
			if _, skip := hopByHop[strings.ToLower(string(k))]; skip {
				return
			}
			ctx.Response.Header.SetBytesKV(k, v)
		})

		// Copy body before releasing the response object
		ctx.SetBody(backendResp.Body())
		fasthttp.ReleaseResponse(backendResp)

		metrics.RequestsTotal.WithLabelValues(fmt.Sprintf("%d", ctx.Response.StatusCode())).Inc()

	case err := <-errChan:
		if proxyErr, ok := err.(*ProxyError); ok {
			ctx.SetStatusCode(proxyErr.Status)
			ctx.SetBodyString(proxyErr.Msg)
			metrics.RequestsTotal.WithLabelValues(fmt.Sprintf("%d", proxyErr.Status)).Inc()
		} else {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBodyString("Unknown error occurred")
			metrics.RequestsTotal.WithLabelValues("500").Inc()
		}

	case <-ctx.Done():
		cancel()
		ctx.SetStatusCode(fasthttp.StatusRequestTimeout)
		ctx.SetBodyString("Request timed out")
		metrics.RequestsTotal.WithLabelValues("504").Inc()
	}
}
