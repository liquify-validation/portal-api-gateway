package proxy

import (
        "bytes"
        "context"
        "fmt"
        "log"
        "time"
		"strings"

        "github.com/valyala/fasthttp"

        "proxy/metrics"
        "proxy/utils"
)

var client = &fasthttp.Client{
        MaxConnsPerHost: 10000,
        ReadTimeout:     30 * time.Second,
        WriteTimeout:    30 * time.Second,
}

// Function to proxy the request to the backend server
// Custom error type that includes the HTTP status code
type ProxyError struct {
        Msg    string
        Status int
}

func (e *ProxyError) Error() string {
        return e.Msg
}

func ProxyHttpRequest(ctx *fasthttp.RequestCtx, req *fasthttp.Request, chain string, chainMap map[string][]string, apiKey string) {
        queryString := string(ctx.QueryArgs().QueryString())
        // Get the input path from the request context
        path := utils.ExtractAdditionalPath(string(ctx.Path()),queryString)

		if req.Header.Peek("X-Forwarded-For") == nil {
			xff := ctx.Request.Header.Peek("X-Forwarded-For")
			if len(xff) > 0 {
				req.Header.Set("X-Forwarded-For", string(xff)) // Copy existing X-Forwarded-For header
			}
		}

		if req.Header.Peek("API-Key") == nil {
			req.Header.Set("API-Key", string(apiKey)) // Add the api key to the header
		}

		acceptHeader := string(ctx.Request.Header.Peek("Accept"))
		isSSE := strings.Contains(acceptHeader, "text/event-stream") || strings.Contains(path, "stream")

		if isSSE {
			proxySSE(chainMap[chain][0] + path, ctx, chain)
			return
		}

        maxRetries := 3
        responseChan := make(chan *fasthttp.Response, 1)
        errChan := make(chan error, 1)

        // Create a cancellable context
        proxyCtx, cancel := context.WithCancel(context.Background())
        defer cancel() // Ensure cancel is called to avoid context leak

        go func() {
                defer close(responseChan)
                defer close(errChan)

                if chainCode, ok := chainMap[chain]; ok {
                        if len(chainCode) != 0 {
                                var lastErr *ProxyError
                                for attempt := 0; attempt < maxRetries; attempt++ {
                                        select {
                                        case <-proxyCtx.Done():
                                                errChan <- &ProxyError{Msg: "request cancelled", Status: fasthttp.StatusRequestTimeout}
                                                return
                                        default:
                                                uri := chainCode[attempt%len(chainCode)] + path
                                                req.SetRequestURI(uri)

                                                backendResp := fasthttp.AcquireResponse()
                                                err := client.Do(req, backendResp)

                                                if err != nil {
                                                        log.Printf("Failed to proxy to: %s, error: %s", uri, err)
                                                        fasthttp.ReleaseResponse(backendResp)
                                                        lastErr = &ProxyError{Msg: err.Error(), Status: fasthttp.StatusBadGateway}
                                                        continue
                                                }

                                                if backendResp.StatusCode() == fasthttp.StatusOK {
                                                        responseChan <- backendResp
                                                        return
                                                }

                                                log.Printf("Failed to proxy to: %s, status code: %d", uri, backendResp.StatusCode())
                                                fasthttp.ReleaseResponse(backendResp)
                                                lastErr = &ProxyError{Msg: fmt.Sprintf("status code: %d", backendResp.StatusCode()), Status: backendResp.StatusCode()}
                                        }
                                }
                                // If all retries fail, send the last error
                                errChan <- lastErr
                                return
                        }
                }
                errChan <- &ProxyError{Msg: "failed to proxy request: invalid chain configuration", Status: fasthttp.StatusBadRequest}
        }()

        select {
        case backendResp := <-responseChan:
                if backendResp != nil {
                        defer fasthttp.ReleaseResponse(backendResp)
                        backendResp.Header.VisitAll(func(key, value []byte) {
                                ctx.Response.Header.Set(string(key), string(value))
                        })
                        ctx.SetBodyStream(bytes.NewReader(backendResp.Body()), len(backendResp.Body()))
                        metrics.RequestsTotal.WithLabelValues(fmt.Sprintf("%d", ctx.Response.StatusCode())).Inc()
                } else {
                        ctx.Error("Error proxying request: no response received", fasthttp.StatusBadGateway)
                        metrics.RequestsTotal.WithLabelValues("502").Inc()
                }
        case err := <-errChan:
                if proxyErr, ok := err.(*ProxyError); ok {
                        ctx.Error(proxyErr.Msg, proxyErr.Status)
                        metrics.RequestsTotal.WithLabelValues(fmt.Sprintf("%d", proxyErr.Status)).Inc()
                } else {
                        ctx.Error("Unknown error occurred", fasthttp.StatusInternalServerError)
                        metrics.RequestsTotal.WithLabelValues("500").Inc()
                }
        case <-ctx.Done():
                cancel() // Cancel the goroutine context
                ctx.Error("Request timed out", fasthttp.StatusRequestTimeout)
                metrics.RequestsTotal.WithLabelValues("504").Inc()
        }
}