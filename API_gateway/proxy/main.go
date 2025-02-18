package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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

var (
	version = "dev"     // Default value, will be overridden at build time
	gitHash = "unknown" // Default value, will be overridden at build time
)

func main() {
	verFlag := flag.Bool("v", false, "Print the version and Git commit hash and exit")
	proxyPort := flag.Int("port.proxy", 80, "Port for the proxy server")
	metricsPort := flag.Int("port.metrics", 9090, "Port for the metrics server")

	// Parse command-line flags
	flag.Parse()

	// If the version flag is passed, print the version and Git hash and exit
	if *verFlag {
		fmt.Printf("API Gateway: %s\n", version)
		fmt.Printf("Git Commit Hash: %s\n", gitHash)
		os.Exit(0)
	}

	// Print welcome message
	fmt.Println("Welcome to the Liquify API Gateway!")
	fmt.Println("This gateway is developed by Liquify LTD.")
	fmt.Println("For any inquiries, please contact contact@liquify.io.")
	fmt.Printf("API Gateway: %s\n", version)
	fmt.Printf("Git Commit Hash: %s\n", gitHash)

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
	proxyAddr := fmt.Sprintf(":%d", *proxyPort)
	go handlers.StartFastHTTPServer(apiCache, usageCache, &usageMutexMap, proxyAddr)

	metricsAddr := fmt.Sprintf(":%d", *metricsPort)
	// Expose Prometheus metrics endpoint
	go startPrometheusServer(metricsAddr)

	// Wait indefinitely
	select {}
}

func startPrometheusServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Error starting Prometheus server: %s", err)
	}
}
