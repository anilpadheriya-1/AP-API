package telemetry

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type LogEntry struct {
	Path         string
	Timestamp    time.Time
	LatencyMs    int64
	StatusCode   int
	PayloadBytes int64
}

type Engine struct {
	db       *sql.DB
	logChan  chan LogEntry
	batchSize int
	wg       sync.WaitGroup
}

func NewEngine(dbConn string, batchSize int) (*Engine, error) {
	db, err := sql.Open("postgres", dbConn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	// Create table if not exists (for demonstration purposes, in prod should use migrations)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS telemetry_logs (
			id SERIAL PRIMARY KEY,
			path VARCHAR(255) NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			latency_ms BIGINT NOT NULL,
			status_code INT NOT NULL,
			payload_bytes BIGINT NOT NULL
		);
	`)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		db:       db,
		logChan:  make(chan LogEntry, 10000), // Buffer to prevent blocking
		batchSize: batchSize,
	}

	e.wg.Add(1)
	go e.worker()

	return e, nil
}

func (e *Engine) Record(entry LogEntry) {
	select {
	case e.logChan <- entry:
	default:
		// Channel full, dropping log to prevent blocking the proxy
		log.Println("telemetry channel full, dropping log entry")
	}
}

func (e *Engine) worker() {
	defer e.wg.Done()

	var batch []LogEntry
	ticker := time.NewTicker(2 * time.Second) // Flush every 2 seconds if batch isn't full
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-e.logChan:
			if !ok {
				// Channel closed, flush remaining and exit
				if len(batch) > 0 {
					e.flush(batch)
				}
				return
			}
			batch = append(batch, entry)
			if len(batch) >= e.batchSize {
				e.flush(batch)
				batch = make([]LogEntry, 0, e.batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				e.flush(batch)
				batch = make([]LogEntry, 0, e.batchSize)
			}
		}
	}
}

func (e *Engine) flush(batch []LogEntry) {
	if len(batch) == 0 {
		return
	}

	// Construct bulk insert query
	valueStrings := make([]string, 0, len(batch))
	valueArgs := make([]interface{}, 0, len(batch)*5)

	i := 0
	for _, entry := range batch {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))
		valueArgs = append(valueArgs, entry.Path, entry.Timestamp, entry.LatencyMs, entry.StatusCode, entry.PayloadBytes)
		i++
	}

	stmt := fmt.Sprintf("INSERT INTO telemetry_logs (path, timestamp, latency_ms, status_code, payload_bytes) VALUES %s", strings.Join(valueStrings, ","))

	_, err := e.db.Exec(stmt, valueArgs...)
	if err != nil {
		log.Printf("Failed to bulk insert telemetry logs: %v\n", err)
	}
}

func (e *Engine) Shutdown() {
	close(e.logChan)
	e.wg.Wait()
	e.db.Close()
}
