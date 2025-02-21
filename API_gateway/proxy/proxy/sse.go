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
				metrics.RequestsTotal.WithLabelValues(strconv.Itoa(200)).Inc()
				metrics.MetricRequestsAPI.WithLabelValues(apikey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(200)).Inc()
				break
			}
		}
	
		// Stream SSE messages
		var partialChunk strings.Builder
		for {
			line, err := reader.ReadString('\n') // Read one line at a time
			if err != nil {
				log.Println("SSE connection closed by upstream:", err)
				return
			}
	
			line = strings.TrimSpace(line) // Remove leading/trailing spaces and newlines
	
			// Skip empty lines
			if line == "" {
				continue
			}
	
			// Skip chunk size headers (hexadecimal lines)
			if matched, _ := regexp.MatchString(`^[0-9a-fA-F]+$`, line); matched {
				continue
			}
	
			// Append valid data
			partialChunk.WriteString(line + "\n") // Add newline since we trimmed it earlier
	
			// Strip chunk encoding headers
			cleanData, remaining := stripChunkHeaders(partialChunk.String())
			partialChunk.Reset()
			partialChunk.WriteString(remaining)
	
			// **Filter out messages containing ":No update available"**
			if strings.Contains(cleanData, ":No update available") {
				continue
			}
	
			// Write and flush cleaned data immediately
			_, writeErr := w.WriteString(cleanData)
			if writeErr != nil {
				log.Println("Error writing SSE data:", writeErr)
				return
			}
			w.Flush()

			metrics.RequestsTotal.WithLabelValues(strconv.Itoa(200)).Inc()
			metrics.MetricRequestsAPI.WithLabelValues(apikey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(200)).Inc()
				
			time.Sleep(10 * time.Millisecond)
		}
	})
}

func stripChunkHeaders(data string) (cleanData, remaining string) {
	lines := strings.Split(data, "\n")
	var output strings.Builder
	inChunk := false

	for _, line := range lines {
		// Detect and skip chunk size headers (hex numbers)
		if matched, _ := regexp.MatchString(`^[0-9a-fA-F]+$`, strings.TrimSpace(line)); matched {
			inChunk = true
			continue
		}

		// Ignore empty lines between chunks
		if inChunk && strings.TrimSpace(line) == "" {
			inChunk = false
			continue
		}

		output.WriteString(line + "\n")
	}

	return output.String(), ""
}

func isHex(str string) bool {
	_, err := strconv.ParseInt(str, 16, 64)
	return err == nil
}
