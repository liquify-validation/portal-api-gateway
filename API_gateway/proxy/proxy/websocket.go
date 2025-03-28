package proxy

import (
	"log"

	"proxy/metrics"
	"strconv"
	"sync"

	"github.com/fasthttp/websocket"
)

func ProxyWebSocketMessages(src, dst *websocket.Conn, apiKey string, keyData map[string]interface{}, done chan struct{}, writeMutex *sync.Mutex) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in WebSocket proxy (backend: %s): %v", apiKey, r)
		}
	}()

	for {
		select {
		case <-done:
			return // Stop processing if the other goroutine has closed
		default:
			messageType, message, err := src.ReadMessage()
			if err != nil {
				log.Printf("Error reading message: %s", err)
				close(done) // Signal other goroutines to stop
				return
			}

			writeMutex.Lock()
			err = dst.WriteMessage(messageType, message)
			writeMutex.Unlock()
			if err != nil {
				log.Printf("Error writing message: %s", err)
				close(done) // Signal other goroutines to stop
				return
			}

			// Log
			metrics.MetricRequestsAPI.WithLabelValues(apiKey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(100)).Inc()
		}
	}
}
