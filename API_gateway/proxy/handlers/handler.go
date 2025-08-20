package handlers

import (
	"log"
	"sync"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/patrickmn/go-cache"
	"github.com/valyala/fasthttp"

	"proxy/config"
	"proxy/metrics"
	"proxy/utils"
	"proxy/database"
)

func StartFastHTTPServer(apiCache *cache.Cache, usageCache *cache.Cache, usageMutexMap *sync.Map, addr string, db *sql.DB) {
	httpEndpoints, wsEndpoints := config.LoadChainMap()

	requestHandler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		if path == "/health" {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("OK")
			return
		}

		apiKey, path, err := utils.ExtractAPIKeyAndPath(ctx)
		if err != nil || apiKey == "" {
			ctx.Error("Forbidden", fasthttp.StatusForbidden)
			return
		}

		cacheEntry, found := apiCache.Get(apiKey)
		if !found {
			cacheEntry, err = database.FetchAPIKeyInfo(db, apiKey)
			if err != nil {
				if err == sql.ErrNoRows {
					ctx.Error("Invalid API key", fasthttp.StatusForbidden)
					metrics.MetricAPICache.WithLabelValues("INVALID").Inc()
				} else {
					ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
				}
				return
			}
			apiCache.Set(apiKey, cacheEntry, 6*time.Hour)
		}

		// Rate limiting
		limit := cacheEntry.(map[string]interface{})["limit"].(int)
		if !utils.IncrementAPIUsage(apiKey, limit, usageCache, usageMutexMap) {
			ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
			return
		}

		// Routing
		if utils.IsWebSocketRequest(ctx) {
			handleWebSocketRequest(ctx, apiKey, wsEndpoints, cacheEntry.(map[string]interface{}))
			return
		}
		handleHTTPRequest(ctx, httpEndpoints, apiKey, path, cacheEntry.(map[string]interface{}), usageCache, usageMutexMap)
	}

	server := &fasthttp.Server{
		Handler:            requestHandler,
		MaxRequestBodySize: 24 * 1024 * 1024, // 24 MM
		ReadBufferSize:     256 * 1024, //256K
	}
	log.Fatal(server.ListenAndServe(addr))
}
