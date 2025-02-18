package proxy

import (
	"bufio"
	"fmt"
	"net"
	"net/url"

	"log"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"proxy/metrics"
)

func proxySSE(target string, ctx *fasthttp.RequestCtx, chain string, apikey string, keyData map[string]interface{}) {
	parsedURL, err := url.Parse(target)
	if err != nil {
		log.Println("Invalid target URL:", err)
		ctx.Error("Invalid target URL", fasthttp.StatusInternalServerError)
		return
	}

	hostPort := parsedURL.Host
	pathAndQuery := parsedURL.RequestURI()

	//Open a raw TCP connection to the host
	conn, err := net.Dial("tcp", hostPort)
	if err != nil {
		log.Println("Failed to connect to upstream SSE:", err)
		ctx.Error("Failed to connect to upstream", fasthttp.StatusBadGateway)
		return
	}

	// Build and send the HTTP request for SSE
	httpRequest := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nAccept: text/event-stream\r\nConnection: keep-alive\r\n\r\n",
		pathAndQuery, parsedURL.Host)

	_, err = conn.Write([]byte(httpRequest))
	if err != nil {
		log.Println("Error writing SSE request:", err)
		ctx.Error("Failed to send SSE request", fasthttp.StatusInternalServerError)
		return
	}

	// Set SSE headers and start streaming using `SetBodyStreamWriter
	ctx.SetContentType("text/event-stream")
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	ctx.Response.Header.Del("Content-Length") // No Content-Length for streaming responses

	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		reader := bufio.NewReader(conn)

		// Read and discard response headers
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Error reading response headers:", err)
				return
			}
			if line == "\r\n" {
				// Assume with each header we have a new packet of data
				metrics.RequestsTotal.WithLabelValues(strconv.Itoa(200)).Inc()
				metrics.MetricRequestsAPI.WithLabelValues(apikey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(200)).Inc()
				break
			}
		}

		// Stream SSE messages
		buf := make([]byte, 4096)
		partialChunk := ""

		for {
			n, err := reader.Read(buf)
			if n > 0 {
				partialChunk += string(buf[:n])
				cleanData, remaining := stripChunkHeaders(partialChunk)
				partialChunk = remaining

				_, writeErr := w.WriteString(cleanData)
				if writeErr != nil {
					log.Println("Error writing SSE data:", writeErr)
					return
				}
				w.Flush()
			}

			if err != nil {
				log.Println("SSE connection closed by upstream:", err)
				return
			}

			time.Sleep(10 * time.Millisecond)
		}
	})
}

func stripChunkHeaders(data string) (string, string) {
	lines := strings.Split(data, "\r\n")
	var cleanLines []string
	remaining := ""

	for i := 0; i < len(lines); i++ {
		if isHex(lines[i]) {
			continue
		}
		cleanLines = append(cleanLines, lines[i])
	}

	if len(lines) > 0 && isHex(lines[len(lines)-1]) {
		remaining = lines[len(lines)-1]
	}

	return strings.Join(cleanLines, "\r\n"), remaining
}

func isHex(str string) bool {
	_, err := strconv.ParseInt(str, 16, 64)
	return err == nil
}
