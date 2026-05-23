package main

import (
	"log"
	"net/http"
	"os"

	"ap-api/auth"
	"ap-api/proxy"
	"ap-api/telemetry"
)

func main() {
	// Configuration (hardcoded for simplicity/example, would normally be from env)
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	postgresConn := os.Getenv("POSTGRES_CONN")
	if postgresConn == "" {
		// Mock Supabase connection
		postgresConn = "user=postgres password=postgres dbname=postgres host=localhost port=5432 sslmode=disable"
	}

	targetURL := os.Getenv("TARGET_URL")
	if targetURL == "" {
		targetURL = "http://httpbin.org" // Default mock upstream
	}

	// 1. Initialize Telemetry Engine
	telemetryEngine, err := telemetry.NewEngine(postgresConn, 100) // Batch 100 logs at a time
	if err != nil {
		log.Fatalf("Failed to initialize telemetry engine: %v", err)
	}
	defer telemetryEngine.Shutdown()

	// 2. Initialize Auth Engine
	authEngine, err := auth.NewAuthEngine(redisAddr)
	if err != nil {
		log.Fatalf("Failed to initialize auth engine: %v", err)
	}

	// 3. Initialize Proxy
	apiProxy, err := proxy.NewProxy(authEngine, telemetryEngine, targetURL)
	if err != nil {
		log.Fatalf("Failed to initialize proxy: %v", err)
	}

	// 4. Start Server
	port := ":8080"
	log.Printf("Starting AP API Gateway on port %s routing to %s", port, targetURL)
	err = http.ListenAndServe(port, apiProxy)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
