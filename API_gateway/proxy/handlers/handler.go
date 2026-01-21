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
	"proxy/database"
	"proxy/metrics"
	"proxy/utils"
)

func StartFastHTTPServer(apiCache *cache.Cache, usageCache *cache.Cache, usageMutexMap *sync.Map, addr string, db *sql.DB) {
	httpEndpoints, wsEndpoints, _ := config.LoadChainMap()

	requestHandler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		if path == "/health" {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("OK")
			return
		}

		// If /api= is present
		if ctx.QueryArgs().Has("api=") {
			apiKey, newPath, err := utils.ExtractAPIKeyAndPath(ctx)
			if err != nil || apiKey == "" {
				ctx.Error("Forbidden", fasthttp.StatusForbidden)
				return
			}
			path = newPath

			cacheEntry, found := apiCache.Get(apiKey)
			if !found {
				var err error
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

			limit := cacheEntry.(map[string]interface{})["limit"].(int)
			if !utils.IncrementAPIUsage(apiKey, limit, usageCache, usageMutexMap) {
				ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
				return
			}

			if utils.IsWebSocketRequest(ctx) {
				handleWebSocketRequest(ctx, apiKey, wsEndpoints, cacheEntry.(map[string]interface{}))
				return
			}
			handleHTTPRequest(ctx, httpEndpoints, apiKey, path, cacheEntry.(map[string]interface{}), usageCache, usageMutexMap)
			return
		}

		// public check, see if /chain/<chain_name>
		chain, extra, ok := utils.ExtractChain(path)
		if !ok {
			ctx.Error("Forbidden", fasthttp.StatusForbidden)
			return
		}

		// Validate chain against DB
		exists, err := database.FetchChainInfo(db, chain)
		if err != nil {
			ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
			return
		}

		// Cache "IP/chain"
		ip := utils.ClientIPFromXFF(ctx)
		key := ip + "/" + chain
		cacheEntry, found := apiCache.Get(key)
		if !found {
			usageCache.Set(key, true, 6*time.Hour)
			cacheEntry = exists
		}

		limit := cacheEntry.(map[string]interface{})["limit"].(int)
		if !utils.IncrementAPIUsage(key, limit, usageCache, usageMutexMap) {
			ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
			return
		}

		handleHTTPRequest(ctx, httpEndpoints, "public", extra, cacheEntry.(map[string]interface{}), usageCache, usageMutexMap)
		return
	}

	server := &fasthttp.Server{
		Handler:            requestHandler,
		MaxRequestBodySize: 24 * 1024 * 1024, // 24 MM
		ReadBufferSize:     256 * 1024,       //256K
	}
	log.Fatal(server.ListenAndServe(addr))
}
