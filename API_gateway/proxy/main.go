package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"proxy/handlers"
	"proxy/metrics"
)

var (
	apiCache      *cache.Cache
	usageCache    *cache.Cache
	usageMutexMap sync.Map
)

func main() {
	// Print welcome message
	fmt.Println("Welcome to the Liquify API Gateway!")
	fmt.Println("This gateway is developed by Liquify LTD.")
	fmt.Println("For any inquiries, please contact contact@liquify.io.")

	// Load environment variables
	if errEnv := godotenv.Load(); errEnv != nil {
		log.Fatalf("Error loading .env file: %s", errEnv)
	}

	// Initialize Prometheus metrics
	metrics.InitPrometheusMetrics()

	// Initialize API and usage caches
	apiCache = cache.New(1*time.Hour, 1*time.Hour)
	usageCache = cache.New(24*time.Hour, 30*time.Minute)

	// Start FastHTTP server to handle requests
	go handlers.StartFastHTTPServer(apiCache, usageCache, &usageMutexMap)

	// Expose Prometheus metrics endpoint
	go startPrometheusServer()

	// Wait indefinitely
	select {}
}

func startPrometheusServer() {
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":29100", nil); err != nil {
		log.Fatalf("Error starting Prometheus server: %s", err)
	}
}
