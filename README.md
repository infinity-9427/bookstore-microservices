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
git clone <repository-url>
cd bookstore-microservices

# Start all services
docker-compose up -d

# Check services status
docker-compose ps
```

## Environment Variables

### Books Service
Create `books/.env`:
```env
BOOKS_DB_DSN=postgresql://books_user:books_password@db:5432/books_db
PORT=8001
```

### Orders Service  
Create `orders/.env`:
```env
# Add orders environment variables as needed
```

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

## Service Ports

- Books API: http://localhost:8001
- Orders API: http://localhost:8082
- PostgreSQL: localhost:5432

## Database

PostgreSQL database with automatic initialization via Docker volumes:
- Database: `books_db`
- User: `books_user` / `books_password`
- Tables auto-created on first run

## Notes

- All services use structured JSON logging
- Books service supports partial updates (only send fields you want to change)
- Soft deletes implemented (records marked inactive, not removed)
- Request ID tracking across services
- CORS enabled for frontend integration