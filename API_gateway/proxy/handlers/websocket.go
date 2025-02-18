package handlers

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"

	"proxy/proxy"
)

func handleWebSocketRequest(ctx *fasthttp.RequestCtx, apiKey string, chainMap map[string][]string, keyData map[string]interface{}) {
	upgrader := websocket.FastHTTPUpgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
	}

	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		defer conn.Close()

		// Set a read deadline to prevent hanging connections (optional)
		conn.SetReadDeadline(time.Now().Add(60 * time.Minute))

		if chainCode, ok := chainMap[keyData["chain"].(string)]; ok {
			if len(chainCode) != 0 {
				//uri := chainCode[0] + path

				backendURL := chainCode[0]

				headers := http.Header{}
				headers.Add("API-Key", apiKey)

				// Handle the X-Forwarded-For header
				xff := ctx.Request.Header.Peek("X-Forwarded-For")
				if len(xff) > 0 {
					headers.Add("X-Forwarded-For", string(xff)) // Copy existing X-Forwarded-For header
				} else {
					headers.Add("X-Forwarded-For", ctx.RemoteIP().String()) // Use client's IP if not present
				}

				backendConn, _, err := websocket.DefaultDialer.Dial(backendURL, headers)
				if err != nil {
					log.Printf("Failed to connect to backend: %s", err)
					return
				}
				defer backendConn.Close()
				// Start proxying messages between the client and backend
				var wg sync.WaitGroup
				wg.Add(2)

				go func() {
					defer wg.Done()
					proxy.ProxyWebSocketMessages(conn, backendConn, apiKey, keyData)
					conn.Close()
				}()
				go func() {
					defer wg.Done()
					proxy.ProxyWebSocketMessages(backendConn, conn, apiKey, keyData)
					backendConn.Close()
				}()

				wg.Wait()
			}
		}
	})

	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
	}
}
