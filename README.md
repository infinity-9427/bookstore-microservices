# Bookstore Microservices Architecture

A modern microservices-based bookstore system built with FastAPI (Python) and Go, featuring concurrent request processing with BullMQ workers and PostgreSQL databases.

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Books API     │    │   Orders API    │    │  BullMQ Workers │
│   (FastAPI)     │    │     (Go)        │    │   (Node.js)     │
│   Port: 8001    │    │   Port: 8082    │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   books_db      │    │   orders_db     │    │     Redis       │
│ (PostgreSQL)    │    │ (PostgreSQL)    │    │   (Message      │
│                 │    │                 │    │    Queue)       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Technology Stack

- **Books Service**: FastAPI (Python) with SQLAlchemy ORM
- **Orders Service**: Go with Gin framework and PGX driver
- **Workers**: Node.js with TypeScript and BullMQ
- **Database**: PostgreSQL (separate databases for each service)
- **Message Queue**: Redis for job processing
- **Monitoring**: Prometheus + Grafana (optional profile)

## 📖 API Documentation

### Books API (FastAPI - Port 8001)

#### Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/health` | Service health check | No |
| `POST` | `/v1/books` | Create a new book | No |
| `GET` | `/v1/books` | List all active books with pagination | No |
| `GET` | `/v1/books/{id}` | Get book by ID | No |
| `PUT` | `/v1/books/{id}` | Update book | No |
| `DELETE` | `/v1/books/{id}` | Soft delete book (sets active=false) | No |

#### Book Model

**Required Fields:**
- `title` (string): Book title
- `author` (string): Book author  
- `description` (string): Book description (supports long text)
- `price` (decimal): Book price (2 decimal places, ≥ 0)

**Optional Fields:**
- `image` (object): Book cover image data
  - `url` (string): Cloudinary image URL
  - `public_id` (string): Cloudinary public ID

**Auto-generated Fields:**
- `id` (integer): Unique book identifier
- `active` (boolean): Soft delete flag (default: true)
- `created_at` (datetime): Creation timestamp
- `updated_at` (datetime): Last update timestamp

#### API Examples & Usage

All prices are returned as strings with exactly 2 decimal places for precision (e.g. "19.99"). Pagination uses `limit` (1-100) and `offset` (≥0). Results are ordered by `created_at DESC` (newest first).

##### Create Book
```bash
curl -i -X POST http://localhost:8001/v1/books \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "The Great Gatsby",
    "author": "F. Scott Fitzgerald",
    "description": "A classic American novel set in the Jazz Age, exploring themes of wealth, love, and the American Dream.",
    "price": "19.99"
  }'
```

Example Response (201):
```json
{
  "id": 1,
  "title": "The Great Gatsby",
  "author": "F. Scott Fitzgerald",
  "description": "A classic American novel set in the Jazz Age, exploring themes of wealth, love, and the American Dream.",
  "price": "19.99",
  "active": true,
  "image": null,
  "created_at": "2023-12-07T10:30:00Z",
  "updated_at": "2023-12-07T10:30:00Z"
}
```

##### Get Book by ID
```bash
curl -s http://localhost:8001/v1/books/1 | jq '.'
```

##### List Books (Paginated)
```bash
curl -i 'http://localhost:8001/v1/books?limit=10&offset=20'
```
Example Response Body:
```json
{
  "data": [
    {
      "id": 3,
      "title": "1984",
      "author": "George Orwell",
      "description": "A dystopian social science fiction novel and cautionary tale.",
      "price": "29.99",
      "active": true,
      "image": null,
      "created_at": "2023-12-07T11:00:00Z",
      "updated_at": "2023-12-07T11:00:00Z"
    }
  ],
  "total": 50,
  "limit": 10,
  "offset": 20
}
```
Important Headers:
```
X-Total-Count: 50
Link: </v1/books?limit=10&offset=30>; rel="next", </v1/books?limit=10&offset=10>; rel="prev"
```

##### Empty Page Example
```bash
curl -s 'http://localhost:8001/v1/books?limit=20&offset=1000'
```
Returns:
```json
{ "data": [], "total": 50, "limit": 20, "offset": 1000 }
```

##### Validation Errors
```bash
# Invalid limit (0)
curl -i 'http://localhost:8001/v1/books?limit=0'

# Invalid offset (-1)
curl -i 'http://localhost:8001/v1/books?offset=-1'
```

##### Update Book (Partial)
```bash
curl -i -X PUT http://localhost:8001/v1/books/1 \
  -H 'Content-Type: application/json' \
  -d '{"price": "24.99", "description": "Updated description."}'
```

##### Soft Delete Book
```bash
curl -i -X DELETE http://localhost:8001/v1/books/1
```

##### cURL Quick Reference (Books)
```bash
# Create
curl -X POST http://localhost:8001/v1/books -H 'Content-Type: application/json' -d '{"title":"T","author":"A","description":"D","price":"9.99"}'
# List (defaults)
curl http://localhost:8001/v1/books
# List (explicit pagination)
curl 'http://localhost:8001/v1/books?limit=5&offset=10'
# Get
curl http://localhost:8001/v1/books/1
# Update
curl -X PUT http://localhost:8001/v1/books/1 -H 'Content-Type: application/json' -d '{"price":"11.49"}'
# Delete (soft)
curl -X DELETE http://localhost:8001/v1/books/1
```

### Orders API (Go - Port 8082)

#### Endpoints

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/health` | Service health check | No |
| `POST` | `/v1/orders` | Create a new order | No |
| `GET` | `/v1/orders` | List all orders | No |
| `GET` | `/v1/orders/{id}` | Get order by ID | No |

#### Order Model

The orders system supports **multiple books per order** using a relational design:

**Order:**
- `id` (integer): Unique order identifier
- `created_at` (datetime): Order creation timestamp
- `items` (array): Array of order items

**Order Item:**
- `id` (integer): Unique item identifier
- `order_id` (integer): Reference to parent order
- `book_id` (integer): Reference to book (validated via Books API)
- `quantity` (integer): Number of books (1-10,000)
- `unit_price` (decimal): Price per book at time of order
- `total_price` (decimal): Calculated total (quantity × unit_price)
- `created_at` (datetime): Item creation timestamp

#### API Examples & Usage

Pagination: `limit` (1-200, default 50, capped at 200), `offset` (≥0). Ordered by `created_at DESC`.

##### Create Order (Single Book)
```bash
curl -i -X POST http://localhost:8082/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"book_id":1,"quantity":2}]}'
```

##### Create Order (Multiple Books)
```bash
curl -i -X POST http://localhost:8082/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"book_id":1,"quantity":2},{"book_id":3,"quantity":1},{"book_id":5,"quantity":3}]}'
```

Example Response (201):
```json
{
  "id": 123,
  "total_price": "64.97",
  "items": [
    {
      "id": 1,
      "order_id": 123,
      "book_id": 1,
      "book_title": "The Great Gatsby",
      "book_author": "F. Scott Fitzgerald",
      "quantity": 2,
      "unit_price": "19.99",
      "total_price": "39.98",
      "created_at": "2023-12-07T10:30:00Z"
    }
  ],
  "created_at": "2023-12-07T10:30:00Z"
}
```
Headers:
```
Location: /v1/orders/123
Content-Type: application/json
```

##### Get Order by ID
```bash
curl -s http://localhost:8082/v1/orders/123 | jq '.'
```

##### List Orders (Paginated)
```bash
curl -i 'http://localhost:8082/v1/orders?limit=20&offset=40'
```
Response Body (truncated):
```json
{
  "data": [
    {
      "id": 2,
      "total_price": "29.99",
      "items": [
        {
          "id": 3,
          "order_id": 2,
          "book_id": 5,
          "book_title": "1984",
          "book_author": "George Orwell",
          "quantity": 1,
          "unit_price": "29.99",
          "total_price": "29.99",
          "created_at": "2023-12-07T11:00:00Z"
        }
      ],
      "created_at": "2023-12-07T11:00:00Z"
    }
  ],
  "total": 150,
  "limit": 20,
  "offset": 40
}
```
Headers:
```
X-Total-Count: 150
Link: </v1/orders?limit=20&offset=60>; rel="next", </v1/orders?limit=20&offset=20>; rel="prev"
```

##### Empty Page
```bash
curl -s 'http://localhost:8082/v1/orders?limit=20&offset=1000'
```
Returns:
```json
{ "data": [], "total": 150, "limit": 20, "offset": 1000 }
```

##### cURL Quick Reference (Orders)
```bash
# Create single-book order
curl -X POST http://localhost:8082/v1/orders -H 'Content-Type: application/json' -d '{"items":[{"book_id":1,"quantity":1}]}'
# Create multi-book order
curl -X POST http://localhost:8082/v1/orders -H 'Content-Type: application/json' -d '{"items":[{"book_id":1,"quantity":2},{"book_id":3,"quantity":1}]}'
# List (defaults)
curl http://localhost:8082/v1/orders
# Paginated list
curl 'http://localhost:8082/v1/orders?limit=10&offset=20'
# Get by ID
curl http://localhost:8082/v1/orders/1
```

## 🚀 Getting Started

### Prerequisites
- Docker & Docker Compose
- pnpm (for workers development)

### Quick Start
```bash
# Clone the repository
git clone <repository-url>
cd bookstore-microservices

# Start all services
docker compose up -d

# With monitoring (optional)
docker compose --profile monitoring up -d
```

### Service URLs
- **Books API**: http://localhost:8001
- **Orders API**: http://localhost:8082  
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090

### Database Access
```bash
# PostgreSQL (both books_db and orders_db)
psql -h localhost -p 5432 -U postgres
```

## 📊 Database Schema

### Books Database (`books_db`)
```sql
CREATE TABLE books (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    author TEXT NOT NULL CHECK (length(btrim(author)) > 0),
    description TEXT NOT NULL CHECK (length(btrim(description)) > 0),
    price NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    image JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Orders Database (`orders_db`)
```sql
-- Orders table (simplified but supports multiple books)
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Order items table for multiple books per order
CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    book_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(10,2) NOT NULL CHECK (unit_price >= 0),
    total_price DECIMAL(10,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## 🔧 Worker System (BullMQ)

The system includes a Node.js worker setup using BullMQ for handling concurrent operations:

### Features
- **Concurrent Processing**: Handle multiple order requests simultaneously
- **Queue Management**: Redis-based job queuing
- **Fault Tolerance**: Job retries and error handling
- **Scalability**: Multiple worker instances (2 replicas by default)

### Worker Commands
```bash
# Development
cd workers
pnpm install
pnpm run dev

# Test job producer
pnpm run test-producer
```

## 🔍 Monitoring

When started with the monitoring profile:
- **Grafana Dashboard**: http://localhost:3000
- **Prometheus Metrics**: http://localhost:9090
- **Health Checks**: All services expose `/health` endpoints

## 📋 Business Logic & Validations

### Books Service
- **Validation**: Title, author, and description cannot be empty
- **Price Precision**: Supports up to 2 decimal places
- **Soft Deletion**: Books are marked as `active=false` instead of physical deletion
- **Image Storage**: Optional Cloudinary integration for book covers

### Orders Service  
- **Book Validation**: Validates book exists and is active via Books API
- **Price Snapshot**: Captures book price at time of order creation
- **Quantity Limits**: 1-10,000 books per item
- **Duplicate Prevention**: Cannot add same book_id multiple times in one order
- **Atomic Operations**: Order and items created in single database transaction

## 🧪 Testing

### Books Service (FastAPI)
```bash
cd books
pytest -v
```

### Orders Service (Go)  
```bash
cd orders
go test ./...
```

### Manual Testing Script
```bash
# 1. Create a book
BOOK_RESPONSE=$(curl -s -X POST http://localhost:8001/v1/books \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Book", "author": "Test Author", "description": "A test book description", "price": "19.99"}')

BOOK_ID=$(echo $BOOK_RESPONSE | jq -r '.id')
echo "Created book with ID: $BOOK_ID"

# 2. Create an order with multiple books
ORDER_RESPONSE=$(curl -s -X POST http://localhost:8082/v1/orders \
  -H "Content-Type: application/json" \
  -d "{\"items\": [{\"book_id\": $BOOK_ID, \"quantity\": 2}]}")

ORDER_ID=$(echo $ORDER_RESPONSE | jq -r '.id')
echo "Created order with ID: $ORDER_ID"

# 3. Verify order
curl -s http://localhost:8082/v1/orders/$ORDER_ID | jq '.'
```

## ⚠️ Assumptions & Limitations

### Current Architecture Assumptions

1. **No Authentication**: All endpoints are publicly accessible
2. **Single Tenant**: No multi-tenancy support
3. **No Inventory Management**: No stock tracking or reservation
4. **Price Changes**: Orders capture price at creation time, subsequent book price changes don't affect existing orders
5. **Soft Deletes**: Books are soft-deleted only (active=false)

### Known Limitations

1. **Deleted Book Scenario**: 
   - **Issue**: If a book is deleted (soft-deleted) after an order is created, the order still references the book_id
   - **Current Behavior**: Order remains valid with captured price information
   - **Implication**: Orders serve as historical records even for deleted books
   - **Workaround**: Book information is snapshotted at order time

2. **No Order Modifications**: 
   - Orders cannot be updated or cancelled once created
   - No inventory validation or reservation

3. **No Payment Processing**: 
   - Orders are created but no payment integration exists
   - No order status management (pending, paid, shipped, etc.)

4. **Cross-Service Data Consistency**:
   - Book price changes don't retroactively affect existing orders (by design)
   - No distributed transaction management between services

5. **Limited Error Handling**:
   - If Books API is down, orders cannot be created
   - No circuit breaker pattern implemented (simplified for demo)

6. **Scalability Considerations**:
   - Database connections not optimized for high concurrency
   - No horizontal scaling strategy documented

### Production Readiness Gaps

To make this production-ready, consider adding:
- Authentication & authorization
- Input validation & sanitization  
- Rate limiting
- Comprehensive error handling
- Health checks with dependencies
- Distributed tracing
- Backup & disaster recovery
- Security headers & HTTPS
- API versioning strategy
- Database migrations management

## 📝 Environment Variables

### Books Service (.env file in books/)
```env
DATABASE_URL=postgresql://postgres:postgres@db/books_db
PORT=8001
LOG_LEVEL=info
```

### Orders Service (.env file in orders/)
```env
DATABASE_URL=postgresql://postgres:postgres@db/orders_db
BOOKS_SERVICE_URL=http://books:8001
PORT=8082
HTTP_TIMEOUT=10s
```

### Workers Service (.env file in workers/)
```env
REDIS_HOST=redis
REDIS_PORT=6379
WORKER_CONCURRENCY=5
```

## 🛠️ Development

### Code Structure
```
├── books/                 # FastAPI Books service
│   ├── main.py           # FastAPI app and routes
│   ├── models.py         # Pydantic models
│   ├── database.py       # SQLAlchemy models
│   └── tests/            # Unit tests
├── orders/               # Go Orders service
│   ├── cmd/api/main.go   # Application entry point
│   ├── internal/         # Internal packages
│   │   ├── handlers/     # HTTP handlers
│   │   ├── service/      # Business logic
│   │   ├── repository/   # Data access
│   │   └── models/       # Data structures
├── workers/              # Node.js Workers service
│   ├── src/
│   │   ├── worker.ts     # BullMQ worker implementation
│   │   └── queue.ts      # Queue definitions
├── docker/               # Docker initialization scripts
│   └── init/             # Database schema and seed data
└── docker-compose.yml    # Service orchestration
```

### Local Development
```bash
# Start dependencies only
docker compose up -d db redis

# Run services locally for development
cd books && python -m uvicorn main:app --reload --port 8001
cd orders && go run cmd/api/main.go
cd workers && pnpm dev
```

---
