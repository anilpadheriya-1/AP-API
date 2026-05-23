package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"ap-api/auth"
	"ap-api/telemetry"
)

type Proxy struct {
	authEngine *auth.AuthEngine
	telemetry  *telemetry.Engine
	targetURL  *url.URL
	client     *http.Client
}

func NewProxy(authEngine *auth.AuthEngine, telemetryEngine *telemetry.Engine, targetURLStr string) (*Proxy, error) {
	target, err := url.Parse(targetURLStr)
	if err != nil {
		return nil, err
	}

	return &Proxy{
		authEngine: authEngine,
		telemetry:  telemetryEngine,
		targetURL:  target,
		client: &http.Client{
			// Minimal overhead client, consider custom transport for higher perf
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (p *Proxy) sendJSONError(w http.ResponseWriter, statusCode int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errorType,
		Message: message,
	})
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	var statusCode int
	var payloadBytes int64

	// Extract custom header
	apiKey := r.Header.Get("X-AP-Key")
	if apiKey == "" {
		statusCode = http.StatusUnauthorized
		p.sendJSONError(w, statusCode, "Unauthorized", "Missing X-AP-Key header in request.")
		p.logTelemetry(r.URL.Path, startTime, statusCode, 0)
		return
	}

	// Validate and rate limit
	authResult, err := p.authEngine.ValidateAndLimit(apiKey)
	if err != nil {
		statusCode = http.StatusInternalServerError
		p.sendJSONError(w, statusCode, "InternalServerError", "An internal error occurred while validating the API key.")
		log.Printf("Auth engine error: %v", err)
		p.logTelemetry(r.URL.Path, startTime, statusCode, 0)
		return
	}

	switch authResult {
	case auth.AuthInvalidKey:
		statusCode = http.StatusUnauthorized
		p.sendJSONError(w, statusCode, "Unauthorized", "The provided X-AP-Key is invalid.")
		p.logTelemetry(r.URL.Path, startTime, statusCode, 0)
		return
	case auth.AuthRateLimited:
		statusCode = http.StatusTooManyRequests
		p.sendJSONError(w, statusCode, "TooManyRequests", "Rate limit exceeded. Maximum 60 requests per minute allowed.")
		p.logTelemetry(r.URL.Path, startTime, statusCode, 0)
		return
	}

	// Forward request to target
	outReq, err := http.NewRequest(r.Method, p.targetURL.String()+r.URL.Path, r.Body)
	if err != nil {
		statusCode = http.StatusInternalServerError
		p.sendJSONError(w, statusCode, "InternalServerError", "Failed to construct upstream request.")
		p.logTelemetry(r.URL.Path, startTime, statusCode, 0)
		return
	}
	outReq.URL.RawQuery = r.URL.RawQuery

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			// Don't forward the AP key
			if key != "X-AP-Key" && key != "X-Ap-Key" {
				outReq.Header.Add(key, value)
			}
		}
	}

	resp, err := p.client.Do(outReq)
	if err != nil {
		statusCode = http.StatusBadGateway
		p.sendJSONError(w, statusCode, "BadGateway", "Failed to connect to the upstream target URL.")
		p.logTelemetry(r.URL.Path, startTime, statusCode, 0)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	statusCode = resp.StatusCode

	// Pipe response body
	var buf bytes.Buffer
	tee := io.TeeReader(resp.Body, &buf)
	copiedBytes, err := io.Copy(w, tee)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
	}
	payloadBytes = copiedBytes

	// Async Logging
	p.logTelemetry(r.URL.Path, startTime, statusCode, payloadBytes)
}

func (p *Proxy) logTelemetry(path string, startTime time.Time, statusCode int, payloadBytes int64) {
	latencyMs := time.Since(startTime).Milliseconds()
	p.telemetry.Record(telemetry.LogEntry{
		Path:         path,
		Timestamp:    startTime,
		LatencyMs:    latencyMs,
		StatusCode:   statusCode,
		PayloadBytes: payloadBytes,
	})
}
