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
		ReadBufferSize:  32768,
		WriteBufferSize: 32768,
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
	}

	err := upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		defer conn.Close()

		conn.SetReadDeadline(0)

		chainName := keyData["chain"].(string)
		chainCode, ok := chainMap[chainName]
		if !ok || len(chainCode) == 0 {
			log.Printf("Invalid chain name or no backend URL for chain: %s", chainName)
			return
		}

		backendURL := chainCode[0]

		headers := http.Header{}
		headers.Add("API-Key", apiKey)

		xff := ctx.Request.Header.Peek("X-Forwarded-For")
		if len(xff) > 0 {
			headers.Add("X-Forwarded-For", string(xff))
		} else {
			headers.Add("X-Forwarded-For", ctx.RemoteIP().String())
		}

		backendConn, _, err := websocket.DefaultDialer.Dial(backendURL, headers)
		if err != nil {
			log.Printf("Failed to connect to backend: %s", err)
			return
		}
		defer backendConn.Close()

		var wg sync.WaitGroup
		done := make(chan struct{})
		writeMutex := &sync.Mutex{}

		// Keepalive ping loop
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					writeMutex.Lock()
					err := conn.WriteMessage(websocket.PingMessage, nil)
					writeMutex.Unlock()
					if err != nil {
						log.Printf("Ping failed, closing connection: %v", err)
						close(done)
						return
					}
				}
			}
		}()

		wg.Add(2)

		go func() {
			defer wg.Done()
			proxy.ProxyWebSocketMessages(conn, backendConn, apiKey, keyData, done, writeMutex)
		}()
		go func() {
			defer wg.Done()
			proxy.ProxyWebSocketMessages(backendConn, conn, apiKey, keyData, done, writeMutex)
		}()

		wg.Wait()
	})

	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
	}
}
