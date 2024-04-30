package main

import (
        "database/sql"
        "fmt"
        "log"
        "net/http"
        "net/url"
        "os"
        "strconv"
        "strings"
        "sync"
        "time"

        "github.com/joho/godotenv"
        _ "github.com/go-sql-driver/mysql"
        "github.com/patrickmn/go-cache"
        "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promhttp"
        "github.com/valyala/fasthttp"
)

var chainMap = map[string]string{
        "eth":     "0021",
        "fuse":    "0005",
        "polygon": "0009",
        "solana":  "C006",
        "bsc":     "0004",
        "base":    "0079",
        "arb":     "0066",
        "dfk":     "03DF",
        "klaytn":  "0056",
}

var db *sql.DB
var apiCache *cache.Cache

type APIUsage struct {
        Count      int64
        LastUpdate time.Time
}

var (
        usageCache = cache.New(24*time.Hour, 30*time.Minute) // Initialize cache with default expiration time and cleanup interval
)

var (
        metricRequestsAPI = prometheus.NewCounterVec(
                prometheus.CounterOpts{
                        Name: "requests_by_api_key",
                        Help: "Number of HTTP requests by API key, organization, organization ID, chain, and status.",
                }, []string{"api_key", "org", "org_id", "chain", "status"},
        )

        metricAPICache = prometheus.NewCounterVec(
                prometheus.CounterOpts{
                        Name: "cache_hits",
                        Help: "Number of calls with cached API key.",
                }, []string{"state"},
        )

        requestsTotal = prometheus.NewCounterVec(
                prometheus.CounterOpts{
                        Name: "http_requests_total",
                        Help: "Total number of HTTP requests.",
                }, []string{"status_code"},
        )
)

// Define a mutex map to store mutexes for each API key
var usageMutexMap sync.Map

func getUsage(apiKey string) *APIUsage {
        // Retrieve the usage for the API key from the cache
        usagePtr, found := usageCache.Get(apiKey)
        if !found {
                return nil
        }
        return usagePtr.(*APIUsage)
}

func setUsage(apiKey string, usage *APIUsage, expire bool) {
        // Update the entry in the cache
		if expire {
        	usageCache.Set(apiKey, usage, 24*time.Hour)
		} else {
			usageCache.Set(apiKey, usage, cache.DefaultExpiration)
		}
}

func getUsageMutex(apiKey string) *sync.Mutex {
        // Retrieve or create a mutex for the specified API key
        mutex, _ := usageMutexMap.LoadOrStore(apiKey, &sync.Mutex{})
        return mutex.(*sync.Mutex)
}

func incrementAPIUsage(apiKey string, limit int) bool {
        // Retrieve the mutex for the specified API key
        usageMutex := getUsageMutex(apiKey)

        // Lock the mutex to ensure exclusive access to the usage value for this API key
        usageMutex.Lock()
        defer usageMutex.Unlock()

		expire := false
        // Load the usage for the API key
        usage := getUsage(apiKey)
        if usage == nil {
                // Initialize usage if not found
                usage = &APIUsage{Count: 1, LastUpdate: time.Now()}
				expire = true
        } else {
                // Increment the usage count
                usage.Count++
				expire = false
        }

        if usage.Count > int64(limit) {
                return false
        }

        log.Printf("API key = %s", usage)

        // Update the entry in the cache
        setUsage(apiKey, usage, expire)
        return true
}

func main() {
        errEnv := godotenv.Load()
        if errEnv != nil {
                log.Fatalf("Error loading .env file: %s", errEnv)
        }

        // Retrieve environment variables
        dbPassword := os.Getenv("DB_PASSWORD")
        dbUser := os.Getenv("DB_USER")
        dbHost := os.Getenv("DB_HOST")
        dbPort := os.Getenv("DB_PORT")
        dbDatabaseName := os.Getenv("DB_NAME")
        proxyHost := os.Getenv("PROXY_HOST")
        proxyPort := os.Getenv("PROXY_PORT")

        // Register custom Prometheus metrics
        prometheus.MustRegister(metricRequestsAPI)
        prometheus.MustRegister(metricAPICache)
        prometheus.MustRegister(requestsTotal)

        apiCache = cache.New(1*time.Hour, 1*time.Hour)

        // Open database connection
        var err error
        db, err = sql.Open("mysql", dbUser+":"+dbPassword+"@tcp("+dbHost+":"+dbPort+")/"+dbDatabaseName)
        if err != nil {
                log.Fatal(err)
        }
        defer db.Close()

        // Initialize request handler
        requestHandler := func(ctx *fasthttp.RequestCtx) {
                // Extract API key from query string
                uri := string(ctx.RequestURI())
                parsedURI, errT := url.Parse(uri)
                if errT != nil {
                        // Handle error
                        return
                }
                path := parsedURI.Path

                apiKey := extractAPIKey(path)
                if apiKey == "" {
                        ctx.Error("Forbidden", fasthttp.StatusForbidden)
                        return
                }

                log.Printf("API key = ", apiKey)

                // Check if API key exists in cache
                if cacheEntry, found := apiCache.Get(apiKey); found {
                        if keyData, ok := cacheEntry.(map[string]interface{}); ok {
                                if limit, ok := keyData["limit"].(int); ok {
                                        if !incrementAPIUsage(apiKey, limit) {
                                                apiCache.Delete(apiKey)
                                                ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
                                                return
                                        }
                                        proxyRequest(ctx, &ctx.Request, proxyHost, proxyPort, keyData["chain"].(string))
                                        metricRequestsAPI.WithLabelValues(apiKey, keyData["org"].(string), keyData["org_id"].(string), keyData["chain"].(string), strconv.Itoa(ctx.Response.StatusCode())).Inc()
                                        return
                                } else {
                                        log.Println("Type assertion failed for limit")
                                        return
                                }
                        } else {
                                log.Println("Type assertion failed for cache entry")
                                return
                        }
                }

                // Check if API key exists in database
                var chain, org string
                var limit, orgID int
                stmt, err := db.Prepare("SELECT chain_name, org_name, `limit`, org_id FROM api_keys WHERE api_key = ?")
                if err != nil {
                    log.Fatal(err)
                }
                defer stmt.Close()
                row := stmt.QueryRow(apiKey)
                err = row.Scan(&chain, &org, &limit, &orgID)
                if err != nil {
                        if err == sql.ErrNoRows {
                                ctx.Error("Invalid API key", fasthttp.StatusForbidden)
                        } else {
                                ctx.Error("Internal server error", fasthttp.StatusInternalServerError)
                        }
                        return
                }

                if !incrementAPIUsage(apiKey, limit) {
                        ctx.Error("Slow down you have hit your daily request limit", fasthttp.StatusTooManyRequests)
                        return
                }

                // Cache API key data
                apiCache.Set(apiKey, map[string]interface{}{
                        "chain": chain, "org": org, "limit": limit, "org_id": strconv.Itoa(orgID),
                }, 1*time.Hour)

                // Proceed with proxy logic
                // Proxy request to backend server
                proxyRequest(ctx, &ctx.Request, proxyHost, proxyPort, chain)
                // Increment API requests metric
                metricRequestsAPI.WithLabelValues(apiKey, org, strconv.Itoa(orgID), chain, strconv.Itoa(ctx.Response.StatusCode())).Inc()
        }

        // Start FastHTTP server
        go func() {
                if err := fasthttp.ListenAndServe(":80", requestHandler); err != nil {
                        log.Fatalf("Error in ListenAndServe: %s", err)
                }
        }()

        // Expose Prometheus metrics endpoint on port 9100
        http.Handle("/metrics", promhttp.Handler())
        if err := http.ListenAndServe(":9100", nil); err != nil {
                log.Fatalf("Error starting Prometheus server: %s", err)
        }
}

// Function to extract API key from query string
func extractAPIKey(queryString string) string {
        // Split query string by '=' to directly extract the API key
        parts := strings.Split(queryString, "=")
        if len(parts) != 2 || parts[0] != "/api" {
                return ""
        }
        return parts[1]
}

// Function to proxy the request to the backend server
func proxyRequest(ctx *fasthttp.RequestCtx, req *fasthttp.Request, host string, port string, chain string) {
        // Create a new HTTP client
        client := &fasthttp.Client{}

        if chainCode, ok := chainMap[chain]; ok {
                uri := "http://" + host + ":" + port + "/relay/" + chainCode
                req.SetRequestURI(uri)

                // Perform the request to the backend server
                backendResp := fasthttp.AcquireResponse()
                defer fasthttp.ReleaseResponse(backendResp)

                if err := client.Do(req, backendResp); err != nil {
                        ctx.Error(fmt.Sprintf("Error proxying request: %s", err), fasthttp.StatusBadGateway)
                        requestsTotal.WithLabelValues("502").Inc()
                        return
                }

                // Set the response headers and body from the backend response
                backendResp.Header.CopyTo(&ctx.Response.Header)
                ctx.Response.SetBody(backendResp.Body())

                // Increment Prometheus metrics
                requestsTotal.WithLabelValues(fmt.Sprintf("%d", ctx.Response.StatusCode())).Inc()
        } else {
                ctx.Error(fmt.Sprintf("Chain does not exist in chainMap"), fasthttp.StatusBadGateway)
                requestsTotal.WithLabelValues("502").Inc()
                return
        }
}