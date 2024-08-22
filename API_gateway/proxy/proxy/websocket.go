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

		// Log the message received
		//log.Printf("Received message of type %d with length %d: %s", messageType, len(message), string(message))

		err = dst.WriteMessage(messageType, message)
		if err != nil {
			log.Printf("Error writing message: %s", err)
			break
		}

		// Log the message sent
		//log.Printf("Sent message of type %d with length %d: %s", messageType, len(message), string(message))
		metrics.MetricRequestsAPI.WithLabelValues(apiKey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(200)).Inc()
	}
}
