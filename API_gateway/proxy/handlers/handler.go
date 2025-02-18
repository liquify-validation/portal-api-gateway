package handlers

import (
	"log"
	"sync"
	"time"

	"database/sql"

	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"github.com/patrickmn/go-cache"
	"github.com/valyala/fasthttp"

	"proxy/config"
	"proxy/metrics"
	"proxy/utils"
)

func StartFastHTTPServer(apiCache *cache.Cache, usageCache *cache.Cache, usageMutexMap *sync.Map, addr string) {
	dbUser, dbPassword, dbHost, dbPort, dbDatabaseName := config.LoadDBConfig()
	httpEndpoints, wsEndpoints := config.LoadChainMap()

	requestHandler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		// Handle /health endpoint
		if path == "/health" {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString("OK")
			return
		}

		apiKey, path, err := utils.ExtractAPIKeyAndPath(ctx)
		if err != nil || apiKey == "" {
			log.Printf("invalid path: %s", path)
			ctx.Error("Forbidden", fasthttp.StatusForbidden)
			return
		}

		if _, found := apiCache.Get(apiKey); !found {
			db, err := sql.Open("mysql", dbUser+":"+dbPassword+"@tcp("+dbHost+":"+dbPort+")/"+dbDatabaseName)
			if err != nil {
				log.Printf("Error opening database connection: %s", err)
				ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
				return
			}
			defer db.Close()

			var chain, org string
			var limit, orgID int
			stmt, err := db.Prepare("SELECT chain_name, org_name, `limit`, org_id FROM api_keys WHERE api_key = ?")
			if err != nil {
				log.Printf("Error in query: %s", err)
				ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
				return
			}
			defer stmt.Close()
			row := stmt.QueryRow(apiKey)
			err = row.Scan(&chain, &org, &limit, &orgID)
			if err != nil {
				if err == sql.ErrNoRows {
					ctx.Error("Invalid API key", fasthttp.StatusForbidden)
					metrics.MetricAPICache.WithLabelValues("INVALID").Inc()
				} else {
					ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
				}
				return
			}

			if !utils.IncrementAPIUsage(apiKey, limit, usageCache, usageMutexMap) {
				ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
				return
			}

			// Cache API key data
			apiCache.Set(apiKey, map[string]interface{}{
				"chain": chain, "org": org, "limit": limit, "org_id": strconv.Itoa(orgID),
			}, 6*time.Hour)
		}

		if cacheEntry, found := apiCache.Get(apiKey); found {
			//check if websocket
			if utils.IsWebSocketRequest(ctx) {
				handleWebSocketRequest(ctx, apiKey, wsEndpoints, cacheEntry.(map[string]interface{}))
				return
			}

			//else http
			handleHTTPRequest(ctx, httpEndpoints, apiKey, path, cacheEntry.(map[string]interface{}), usageCache, usageMutexMap)

		}
	}

	if err := fasthttp.ListenAndServe(addr, requestHandler); err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}
