package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"database/sql"
	"log"
	"strings"
	"sync"
	"time"
	"fmt"
)

type MetricsBuffer struct {
	mu     sync.Mutex
	buffer map[string]int64
	db     *sql.DB
}

func NewMetricsBuffer(db *sql.DB) *MetricsBuffer {
	return &MetricsBuffer{
		buffer: make(map[string]int64),
		db:     db,
	}
}

func (m *MetricsBuffer) IncrementBuffered(apiKey, org, orgID, chain, status, method, region string) {
    bucket := time.Now().Truncate(time.Minute).Format(time.RFC3339)
    key := bucket + "|" + apiKey + "|" + org + "|" + orgID + "|" + chain + "|" + status + "|" + method + "|" + region

    m.mu.Lock()
    m.buffer[key]++
    m.mu.Unlock()
}

func (m *MetricsBuffer) StartFlusher(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Flush()
		case <-stopCh:
			m.Flush() // final flush on shutdown
			return
		}
	}
}

// Flush writes buffered metrics to TimescaleDB and resets the buffer.
func (m *MetricsBuffer) Flush() {
    m.mu.Lock()
    if len(m.buffer) == 0 {
        m.mu.Unlock()
        return
    }
    bufferCopy := m.buffer
    m.buffer = make(map[string]int64)
    m.mu.Unlock()

    // Build one multi-row insert
    values := make([]interface{}, 0, len(bufferCopy)*9)
    placeholders := make([]string, 0, len(bufferCopy))

    i := 1
    for key, count := range bufferCopy {
        parts := strings.Split(key, "|")
        if len(parts) != 8 {
            continue
        }
        bucketTime, _ := time.Parse(time.RFC3339, parts[0])
        placeholders = append(placeholders, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
            i, i+1, i+2, i+3, i+4, i+5, i+6, i+7, i+8))
        values = append(values,
            bucketTime,
            parts[1], parts[2], parts[3], parts[4], parts[5], parts[6], parts[7],
            count,
        )
        i += 9
    }

    query := `
        INSERT INTO api_requests (time, api_key, org, org_id, chain, status, method, region, count)
        VALUES ` + strings.Join(placeholders, ",") + `
        ON CONFLICT (time, api_key, org, org_id, chain, status, method, region)
        DO UPDATE SET count = api_requests.count + EXCLUDED.count;
    `

    _, err := m.db.Exec(query, values...)
    if err != nil {
        log.Printf("failed to batch insert metrics: %v", err)
    }
}


var (
	MetricRequestsAPI = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_by_api_key",
			Help: "Number of HTTP requests by API key, organization, organization ID, chain, and status.",
		}, []string{"api_key", "org", "org_id", "chain", "status"},
	)

	MetricAPICache = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits",
			Help: "Number of calls with cached API key.",
		}, []string{"state"},
	)

	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"status_code"},
	)
)

func InitPrometheusMetrics() {
	prometheus.MustRegister(MetricRequestsAPI)
	prometheus.MustRegister(MetricAPICache)
	prometheus.MustRegister(RequestsTotal)
}


