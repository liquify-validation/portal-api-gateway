package proxy

import (
	"log"

	"proxy/metrics"
	"strconv"

	"github.com/fasthttp/websocket"
)

func ProxyWebSocketMessages(src, dst *websocket.Conn, apiKey string, keyData map[string]interface{}) {
	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				log.Printf("WebSocket closed unexpectedly: %s", err)
			} else {
				log.Printf("Error reading message: %s", err)
			}
			break
		}

		err = dst.WriteMessage(messageType, message)
		if err != nil {
			log.Printf("Error writing message: %s", err)
			break
		}

		// Log
		metrics.MetricRequestsAPI.WithLabelValues(apiKey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(100)).Inc()
	}
}
