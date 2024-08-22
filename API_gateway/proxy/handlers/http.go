package handlers

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/valyala/fasthttp"

	"proxy/metrics"
	"proxy/proxy"
	"proxy/utils"
)

func handleHTTPRequest(ctx *fasthttp.RequestCtx, chainMap map[string][]string, apiKey string, path string, keyData map[string]interface{}, usageCache *cache.Cache, usageMutexMap *sync.Map) {
	timeoutDuration := 20 * time.Second

	// Create a channel to signal the completion of the request
	done := make(chan struct{}, 1)

	go func() {
		setCORSHeaders := func() {
			if len(ctx.Response.Header.Peek("Access-Control-Allow-Origin")) == 0 {
				ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
			}

			if len(ctx.Response.Header.Peek("Access-Control-Allow-Methods")) == 0 {
				ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			}

			if len(ctx.Response.Header.Peek("Access-Control-Allow-Headers")) == 0 {
				ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, solana-client")
			}
		}

		setCORSHeaders()

		handleCachedAPIKey(ctx, apiKey, keyData, chainMap, usageCache, usageMutexMap)

		done <- struct{}{}
	}()

	// Wait for either the request to complete or the timeout
	select {
	case <-done:
		// Request completed successfully within the timeout
	case <-time.After(timeoutDuration):
		// Timeout reached, send a timeout response
		ctx.Error("Request timed out", fasthttp.StatusRequestTimeout)
	}
}

// handleCachedAPIKey handles requests with cached API key
func handleCachedAPIKey(ctx *fasthttp.RequestCtx, apiKey string, keyData map[string]interface{}, chainMap map[string][]string, usageCache *cache.Cache, usageMutexMap *sync.Map) {
	// Check if all required keys exist in the keyData map
	requiredKeys := []string{"limit", "chain", "org", "org_id"}
	for _, key := range requiredKeys {
		if _, ok := keyData[key]; !ok {
			log.Printf("Key '%s' not found in keyData", key)
			return
		}
	}

	// Convert the "limit" value to an int
	limit, ok := keyData["limit"].(int)
	if !ok {
		log.Println("Value associated with 'limit' key is not of type int")
		return
	}

	// Proceed with the request handling
	if !utils.IncrementAPIUsage(apiKey, limit, usageCache, usageMutexMap) {
		usageCache.Delete(apiKey)
		ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
		return
	}

	proxy.ProxyHttpRequest(ctx, &ctx.Request, keyData["chain"].(string), chainMap)
	metrics.MetricRequestsAPI.WithLabelValues(apiKey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(ctx.Response.StatusCode())).Inc()
	metrics.MetricAPICache.WithLabelValues("HIT").Inc()
}
