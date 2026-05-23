# AP API Gateway

A hyper-scalable B2B Developer Infrastructure proxy platform.

## Architecture Highlights
- **Proxy Layer**: Built in Go for compile-time optimization, concurrency handling, and low memory usage.
- **Authentication/Rate-limiting**: Redis-backed with sub-millisecond key lookups. Uses a token-bucket (sliding window) implementation. Limit: 60 requests/minute.
- **Persistent Data & Analytics**: PostgreSQL (mocking Supabase) with out-of-band asynchronous worker queue batch bulk-inserts.

## Quickstart

This application requires Docker and Docker Compose.

1. **Start the environment:**
   ```bash
   docker-compose up --build -d
   ```
   This command starts the API Gateway on port `8080`, Redis on port `6379`, and PostgreSQL on `5432`.

2. **Test Unauthorized Request:**
   ```bash
   curl -i http://localhost:8080/get
   ```
   *Expected Response:* `401 Unauthorized` with JSON developer-friendly payload.

3. **Test Authorized Request:**
   ```bash
   curl -i -H "X-AP-Key: key_mock123" http://localhost:8080/get
   ```
   *Expected Response:* `200 OK` (piped from httpbin.org)

4. **Test Rate Limiter (send >60 requests within a minute):**
   ```bash
   for i in {1..65}; do curl -s -o /dev/null -w "%{http_code}\n" -H "X-AP-Key: key_mock123" http://localhost:8080/get; done
   ```
   *Expected Response after 60 reqs:* `429 Too Many Requests`

## System Error Output Standard
All gateway-level errors follow standard developer experience formatting:
```json
{
  "error": "ErrorType",
  "message": "Human readable message describing failure."
}
```
