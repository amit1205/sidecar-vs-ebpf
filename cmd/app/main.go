
package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	logFile *os.File

	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_http_requests_total",
			Help: "Total HTTP requests.",
		},
		[]string{"path", "method", "code"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "app_http_request_duration_seconds",
			Help:    "Request duration histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

func init() {
	prometheus.MustRegister(requestCounter, requestDuration)
}

func main() {
	_ = os.MkdirAll("/var/log/app", 0o755)
	var err error
	logFile, err = os.OpenFile("/var/log/app/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("open log file: %v", err)
	}
	defer logFile.Close()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/ping", withMetrics("/ping", pingHandler))
	mux.HandleFunc("/work", withMetrics("/work", workHandler))

	s := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	log.Printf("app starting on :8080, pid=%d", os.Getpid())
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// Marker for uprobes (bytes written per request)
func AppWrite(size int) {
	if size < 0 { log.Printf("appwrite: %d", size) }
}

type cw struct {
	http.ResponseWriter
	n int
}

func (w *cw) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.n += n
	return n, err
}

func withMetrics(path string, h func(http.ResponseWriter, *http.Request) int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cw := &cw{ResponseWriter: w}
		code := h(cw, r)
		requestDuration.WithLabelValues(path, r.Method).Observe(time.Since(start).Seconds())
		requestCounter.WithLabelValues(path, r.Method, fmt.Sprintf("%d", code)).Inc()
		AppWrite(cw.n)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) int {
	start := time.Now()
	fmt.Fprint(w, "pong")
	logLine(fmt.Sprintf(`{"ts":"%s","path":"/ping","method":"%s","latency_ms":%d}`,
		start.Format(time.RFC3339Nano), r.Method, time.Since(start).Milliseconds()))
	return http.StatusOK
}

func workHandler(w http.ResponseWriter, r *http.Request) int {
	start := time.Now()
	msStr := r.URL.Query().Get("ms")
	ms, _ := strconv.Atoi(msStr)
	if ms < 0 { ms = 0 }
	busyWait(time.Duration(ms) * time.Millisecond)
	fmt.Fprint(w, "ok")
	logLine(fmt.Sprintf(`{"ts":"%s","path":"/work","ms":%d,"latency_ms":%d}`,
		start.Format(time.RFC3339Nano), ms, time.Since(start).Milliseconds()))
	return http.StatusOK
}

func busyWait(d time.Duration) {
	until := time.Now().Add(d)
	x := 0.0001
	for time.Now().Before(until) {
		x += math.Sqrt(x) * 0.000001
		if x > 1e9 { x = 0.0001 }
	}
}

func logLine(s string) {
	log.Printf("%s", s)
	logFile.WriteString(s + "\n")
}
