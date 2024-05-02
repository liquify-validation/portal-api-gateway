# Liquify API Gateway

This API Gateway, written in Go, acts as a middleware between clients and pokt gateway servers. It verifies API keys stored in a MySQL database, caches them for efficiency, enforces rate limiting, and forwards requests to backend servers. The gateway also logs Prometheus metrics, tracking requests by API key, cache hits, and total HTTP requests, along with rate limiting information.

## Features

- **API Key Verification**: Keys passed in the path (`/api=<key>`) are verified against a MySQL database.
- **Caching**: Validated keys are cached for 1 hour to minimize database queries and improve performance.
- **Rate Limiting**: Requests from each API key are rate-limited, returning a 429 status code if the limit is breached.
- **Forwarding**: Requests with valid keys are forwarded to backend servers, and responses are sent back to the caller.
- **Prometheus Metrics**: Metrics are logged on a per API key basis, including requests by API key, cache hits, and total HTTP requests, providing insights into gateway performance.

## Prerequisites

- Go 1.22 or higher
- MySQL database
- FastHTTP library for HTTP request handling
- Prometheus for metrics collection

## Usage

1. Clients should make requests to the API gateway with the API key appended to the path (e.g., `/api=<key>`).

2. The gateway verifies the key against the MySQL database. If valid, it caches the key for 1 hour.

3. Requests from each API key are rate-limited. If the limit is breached, a 429 status code is returned.

4. Valid requests are forwarded to backend servers, and responses are returned to the caller.

## Prometheus Metrics

- **requests_by_api_key**: Number of requests received by the gateway per API key.
- **cache_hits**: Number of cache hits.
- **http_requests_total**: Total number of HTTP requests received by the gateway.
