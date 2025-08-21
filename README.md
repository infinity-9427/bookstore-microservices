# Bookstore Microservices

A distributed bookstore application built with microservices architecture.

## Tech Stack

- **Books Service**: Python 3.11+ with FastAPI, SQLAlchemy, PostgreSQL
- **Orders Service**: Go 1.24+ with Gin framework, PostgreSQL  
- **Workers**: Node.js with TypeScript, BullMQ, Redis
- **Database**: PostgreSQL 16
- **Container**: Docker & Docker Compose

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Git

### Clone & Run

```bash
# Clone repository
git clone <https://github.com/infinity-9427/bookstore-microservices-api.git>
cd bookstore-microservices-api

# Start all services
docker-compose up -d

# Check services status
docker-compose ps
```

## Environment Variables

Before running the services, you need to create `.env` files for each microservice. Example files are provided as `.env.example` in each service directory.

### Books Service
Copy and configure `books/.env.example` to `books/.env`:
```bash
cp books/.env.example books/.env
```

Required variables:
- `BOOKS_DB_DSN`: PostgreSQL connection string for books database
- `PORT`: Port number for the books service (default: 8001)

### Orders Service  
Copy and configure `orders/.env.example` to `orders/.env`:
```bash
cp orders/.env.example orders/.env
```

Required variables:
- `DATABASE_URL`: PostgreSQL connection string for orders database
- `BOOKS_SERVICE_URL`: URL of the books service for inter-service communication
- `PORT`: Port number for the orders service (default: 8082)

## Local Development

### With Docker Compose (Recommended)
```bash
# Build and start all services
docker-compose up --build

# Start specific service
docker-compose up books

# View logs
docker-compose logs -f books

# Stop services
docker-compose down
```

### Individual Services

#### Books Service
```bash
cd books
python -m pip install -e .
uvicorn main:app --host 0.0.0.0 --port 8001 --reload
```

#### Orders Service
```bash
cd orders
go run cmd/api/main.go
```

#### Workers
```bash
cd workers
pnpm install
pnpm dev
```

## API Endpoints

### Books Service (Port 8001)

#### Get All Books
```bash
GET /v1/books
curl http://localhost:8001/v1/books
```

Response:
```json
[
  {
    "id": 1,
    "title": "The Great Gatsby",
    "author": "F. Scott Fitzgerald",
    "price": "12.99",
    "active": true,
    "created_at": "2025-08-21T01:25:12.438013Z",
    "updated_at": "2025-08-21T01:25:12.438013Z"
  }
]
```

#### Get Book by ID
```bash
GET /v1/books/{id}
curl http://localhost:8001/v1/books/1
```

#### Create Book
```bash
POST /v1/books
curl -X POST http://localhost:8001/v1/books \
  -H "Content-Type: application/json" \
  -d '{
    "title": "1984",
    "author": "George Orwell", 
    "price": "15.99"
  }'
```

#### Update Book (Partial Updates Supported)
```bash
PUT /v1/books/{id}
curl -X PUT http://localhost:8001/v1/books/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Animal Farm",
    "author": "George Orwell"
  }'
```

#### Delete Book (Soft Delete)
```bash
DELETE /v1/books/{id}
curl -X DELETE http://localhost:8001/v1/books/1
```

#### Health Check
```bash
GET /health
curl http://localhost:8001/health
```

### Orders Service (Port 8082)

#### Create Order
```bash
POST /v1/orders
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "book_id": 1,
    "quantity": 2
  }'
```

Response:
```json
{
  "id": 1,
  "book_id": 1,
  "book_title": "The Great Gatsby",
  "book_author": "F. Scott Fitzgerald",
  "quantity": 2,
  "unit_price": 12.99,
  "total_price": 25.98,
  "created_at": "2025-08-21T11:58:24.399061Z"
}
```

#### Get All Orders
```bash
GET /v1/orders
curl http://localhost:8082/v1/orders
```

Response:
```json
[
  {
    "id": 1,
    "book_id": 1,
    "book_title": "The Great Gatsby",
    "book_author": "F. Scott Fitzgerald",
    "quantity": 2,
    "unit_price": 12.99,
    "total_price": 25.98,
    "created_at": "2025-08-21T11:58:24.399061Z"
  }
]
```

#### Get Order by ID
```bash
GET /v1/orders/{id}
curl http://localhost:8082/v1/orders/1
```

#### Health Check
```bash
GET /health
curl http://localhost:8082/health
```

Response:
```json
{
  "status": "healthy",
  "services": {
    "database": "healthy",
    "books": "healthy"
  }
}
```

## Service Ports

- Books API: http://localhost:8001
- Orders API: http://localhost:8082
- PostgreSQL: localhost:5432
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

## Database

PostgreSQL database with automatic initialization via Docker volumes:
- **Books Database**: `books_db` 
  - User: `books_user` / `books_password`
- **Orders Database**: `orders_db`
  - User: `orders_user` / `orders_password`
- Tables auto-created on first run

## Architecture

### Service Independence
- **Books Service**: FastAPI with independent PostgreSQL database
- **Orders Service**: Go/Gin with independent PostgreSQL database
- **Communication**: HTTP-only, no shared database access
- **Data Strategy**: Snapshot pattern - orders store book details at creation time

### Key Features
- **Circuit Breaker**: Orders service resilient to Books service failures
- **Health Checks**: Deep health monitoring with dependency status
- **Structured Logging**: JSON logs with request correlation IDs
- **API Versioning**: `/v1/` prefix for future compatibility
- **Prometheus Metrics**: Performance monitoring for both services
- **Graceful Shutdown**: Proper cleanup and signal handling

## Monitoring with Prometheus & Grafana

Start the monitoring stack with:
```bash
docker compose --profile monitoring up -d
```

This provides:
- **Prometheus**: Metrics collection at http://localhost:9090
- **Grafana**: Dashboards at http://localhost:3000 (admin/admin)
- **Golden Signals**: Service health, request rate, error rate, latency

### Monitoring URLs
- Prometheus UI: http://localhost:9090
- Grafana Dashboards: http://localhost:3000 (login: admin/admin)
- Orders Service Metrics: http://localhost:8082/metrics

### Key Prometheus Queries for Orders Service

#### Service Availability
```promql
# Check if Orders service is up
up{job="orders"}

# Alert when service is down
up{job="orders"} == 0
```

#### Request Rate (RPS)
```promql
# Total requests per second
sum(rate(http_requests_total{job="orders"}[5m]))

# Requests per second by endpoint
sum(rate(http_requests_total{job="orders"}[5m])) by (handler)

# Requests per second by method
sum(rate(http_requests_total{job="orders"}[5m])) by (method)
```

#### Error Rate
```promql
# 5xx error rate percentage
sum(rate(http_requests_total{job="orders",status_class="5xx"}[5m])) / sum(rate(http_requests_total{job="orders"}[5m])) * 100

# 4xx error rate percentage  
sum(rate(http_requests_total{job="orders",status_class="4xx"}[5m])) / sum(rate(http_requests_total{job="orders"}[5m])) * 100

# Total error rate (4xx + 5xx)
sum(rate(http_requests_total{job="orders",status_class=~"4xx|5xx"}[5m])) / sum(rate(http_requests_total{job="orders"}[5m])) * 100
```

#### Response Latency
```promql
# P95 latency (95th percentile)
histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{job="orders"}[5m])))

# P50 latency (median)
histogram_quantile(0.50, sum by (le) (rate(http_request_duration_seconds_bucket{job="orders"}[5m])))

# P99 latency
histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="orders"}[5m])))

# Average latency by endpoint
sum(rate(http_request_duration_seconds_sum{job="orders"}[5m])) by (handler) / sum(rate(http_request_duration_seconds_count{job="orders"}[5m])) by (handler)
```

### Grafana Dashboard Setup

1. **Access Grafana**: http://localhost:3000 (admin/admin)
2. **Add Prometheus Data Source**:
   - Go to Configuration → Data Sources
   - Add Prometheus with URL: http://prometheus:9090
3. **Create Dashboard** with panels for:
   - Service uptime
   - Request rate (RPS)
   - Error rate percentage
   - Response latency percentiles
   - Active connections

### Alerting Rules

The system includes predefined alerts in `monitoring/rules/alerts.yml`:

#### OrdersServiceDown
- **Condition**: `up{job="orders"} == 0`
- **Duration**: 1 minute
- **Severity**: Critical
- **Description**: Triggers when Prometheus can't scrape the orders service

#### OrdersHigh5xxRate  
- **Condition**: 5xx error rate > 10%
- **Duration**: 5 minutes
- **Severity**: Warning
- **Description**: Triggers when error rate exceeds threshold

#### OrdersHighLatencyP95
- **Condition**: P95 latency > 1 second
- **Duration**: 5 minutes  
- **Severity**: Warning
- **Description**: Triggers when response times are too slow

## API Testing Guide

### Complete API Testing Examples

#### 1. End-to-End Workflow with cURL

```bash
# Step 1: Create a book first
curl -X POST http://localhost:8001/v1/books \
  -H "Content-Type: application/json" \
  -d '{
    "title": "The Catcher in the Rye", 
    "author": "J.D. Salinger", 
    "price": 18.99
  }'

# Expected response: {"id": 1, "title": "The Catcher in the Rye", ...}

# Step 2: Create an order for the book
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "book_id": 1, 
    "quantity": 3
  }'

# Expected response: Order with book details snapshotted

# Step 3: Verify the order
curl http://localhost:8082/v1/orders/1

# Step 4: List all orders
curl http://localhost:8082/v1/orders

# Step 5: Check health status
curl http://localhost:8082/health
```

#### 2. Postman Collection Examples

**Create Order (POST /v1/orders)**
```json
{
  "method": "POST",
  "url": "{{base_url}}/v1/orders",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "book_id": 1,
    "quantity": 2
  }
}
```

**Environment Variables for Postman:**
- `base_url`: `http://localhost:8082`
- `books_url`: `http://localhost:8001`

#### 3. Advanced Testing Scenarios

**Load Testing with Multiple Orders:**
```bash
# Create multiple orders rapidly
for i in {1..10}; do
  curl -X POST http://localhost:8082/v1/orders \
    -H "Content-Type: application/json" \
    -d "{\"book_id\": 1, \"quantity\": $i}" &
done
wait

# Check all orders were created
curl http://localhost:8082/v1/orders
```

**Circuit Breaker Testing:**
```bash
# Stop books service to test circuit breaker
docker stop $(docker ps -q --filter name=books)

# Try to create order (should fail gracefully)
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"book_id": 1, "quantity": 2}' \
  -w "\nHTTP Status: %{http_code}\nResponse Time: %{time_total}s\n"

# Check health endpoint (should show books service as unhealthy)
curl http://localhost:8082/health | jq .

# Restart books service
docker start $(docker ps -aq --filter name=books)
```

### Error Handling & Troubleshooting

#### Common API Errors

**1. Validation Errors (400 Bad Request)**
```bash
# Invalid book_id (negative number)
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"book_id": -1, "quantity": 2}'

# Invalid quantity (zero or negative)
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"book_id": 1, "quantity": 0}'

# Missing required fields
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"book_id": 1}'
```

**2. Resource Not Found (404)**
```bash
# Book doesn't exist
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"book_id": 999, "quantity": 2}'

# Order doesn't exist
curl http://localhost:8082/v1/orders/999
```

**3. Service Unavailable (503)**
```bash
# When books service is down
docker stop $(docker ps -q --filter name=books)
curl -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"book_id": 1, "quantity": 2}'
```

#### Troubleshooting with Monitoring

When API calls fail, check these monitoring sources:

**1. Check Service Health**
```bash
# Orders service health
curl http://localhost:8082/health

# Expected healthy response:
{
  "status": "healthy",
  "services": {
    "database": "healthy", 
    "books": "healthy"
  }
}
```

**2. Review Prometheus Metrics**
Visit http://localhost:9090 and run these queries:

```promql
# Check if services are running
up{job="orders"}

# Recent error rate
sum(rate(http_requests_total{job="orders",status_class="5xx"}[5m]))

# High latency requests
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{job="orders"}[5m])))

# Failed requests by endpoint
sum(rate(http_requests_total{job="orders",status_class=~"4xx|5xx"}[5m])) by (handler)
```

**3. Check Grafana Dashboards**
Visit http://localhost:3000 (admin/admin) to view:
- Service uptime graphs
- Request rate trends
- Error rate spikes
- Latency percentiles
- Alert status

**4. Review Application Logs**
```bash
# Orders service logs
docker compose logs -f orders

# Filter for errors
docker compose logs orders | grep -E "(ERROR|WARN|error|failed)"

# Books service logs (if needed)
docker compose logs -f books
```

**5. Database Connection Issues**
```bash
# Check database connectivity
docker compose exec orders-db psql -U orders_user -d orders_db -c "SELECT 1;"

# Check database logs
docker compose logs db
```

#### Performance Debugging

**Identify Slow Endpoints:**
```promql
# Average response time by endpoint (last 5 minutes)
sum(rate(http_request_duration_seconds_sum{job="orders"}[5m])) by (handler) 
/ 
sum(rate(http_request_duration_seconds_count{job="orders"}[5m])) by (handler)
```

**Find High Error Rate Endpoints:**
```promql
# Error rate by endpoint
sum(rate(http_requests_total{job="orders",status_class=~"4xx|5xx"}[5m])) by (handler)
/ 
sum(rate(http_requests_total{job="orders"}[5m])) by (handler) * 100
```

**Monitor Resource Usage:**
```bash
# Container resource usage
docker stats

# Detailed container metrics
docker inspect orders | grep -A 10 "Memory"
```

