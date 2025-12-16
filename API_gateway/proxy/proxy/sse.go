package proxy

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"proxy/metrics"

	"github.com/valyala/fasthttp"
)

const (
	upstreamReaderSize = 256 * 1024
	maxEventSize       = 4 * 1024 * 1024 // 4MB safety cap
)

func proxySSE(target string, ctx *fasthttp.RequestCtx, chain string, apikey string, keyData map[string]interface{}) {
	parsedURL, err := url.Parse(target)
	if err != nil {
		log.Println("Invalid target URL:", err)
		ctx.Error("Invalid target URL", fasthttp.StatusInternalServerError)
		return
	}

	conn, err := net.Dial("tcp", parsedURL.Host)
	if err != nil {
		log.Println("Failed to connect upstream:", err)
		ctx.Error("Failed to connect upstream", fasthttp.StatusBadGateway)
		return
	}

	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nAccept: text/event-stream\r\nCache-Control: no-cache\r\nConnection: keep-alive\r\n\r\n",
		parsedURL.RequestURI(),
		parsedURL.Host,
	)

	if _, err := conn.Write([]byte(req)); err != nil {
		log.Println("Failed to write upstream request:", err)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("text/event-stream; charset=utf-8")
	ctx.Response.Header.Set("Cache-Control", "no-cache, no-transform")
	ctx.Response.Header.Set("Connection", "keep-alive")
	ctx.Response.Header.Set("X-Accel-Buffering", "no")
	ctx.Response.Header.Del("Content-Length")

	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer conn.Close()

		reader := bufio.NewReaderSize(conn, upstreamReaderSize)

		// Skip upstream headers
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Header read error:", err)
				return
			}
			if line == "\r\n" {
				break
			}
		}

		metrics.RequestsTotal.WithLabelValues("200").Inc()
		metrics.MetricRequestsAPI.WithLabelValues(
			apikey,
			keyData["org"].(string),
			keyData["org_id"].(string),
			keyData["chain"].(string),
			"200",
		).Inc()

		var (
			eventBuf strings.Builder
			prevByte byte
		)

		for {
			b, err := reader.ReadByte()
			if err != nil {
				if err != io.EOF {
					log.Println("Upstream read error:", err)
				}
				return
			}

			eventBuf.WriteByte(b)
			if eventBuf.Len() > maxEventSize {
				log.Println("SSE event exceeded max size, dropping")
				eventBuf.Reset()
				continue
			}

			// Detect "\n\n" (end of SSE event)
			if prevByte == '\n' && b == '\n' {
				event := eventBuf.String()
				eventBuf.Reset()

				if strings.Contains(event, ":No update available") {
					prevByte = 0
					continue
				}

				if _, err := w.WriteString(event); err != nil {
					log.Println("Write error:", err)
					return
				}
				if err := w.Flush(); err != nil {
					log.Println("Flush error:", err)
					return
				}

				metrics.RequestsTotal.WithLabelValues("200").Inc()
				metrics.MetricRequestsAPI.WithLabelValues(
					apikey,
					keyData["org"].(string),
					keyData["org_id"].(string),
					keyData["chain"].(string),
					"200",
				).Inc()

				time.Sleep(5 * time.Millisecond)
			}

			prevByte = b
		}
	})
}
