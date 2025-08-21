import logging
import uuid
import json
import time
from typing import List
from fastapi import FastAPI, HTTPException, Depends, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from fastapi.exceptions import RequestValidationError
from sqlalchemy.orm import Session
from sqlalchemy import select
from sqlalchemy.exc import SQLAlchemyError

from models import BookRequest, BookResponse, BookUpdateRequest
from database import Book
from db_session import get_db_session
from config import get_config

from prometheus_fastapi_instrumentator import Instrumentator

app = FastAPI(title="Books API", version="1.0.0")

instrumentator = Instrumentator(
    excluded_handlers=["/health", "/metrics", "/docs", "/openapi.json"],
    env_var_name="ENABLE_METRICS",
    inprogress_name="http_requests_inprogress",
    inprogress_labels=True,
)
instrumentator.instrument(app).expose(app, endpoint="/metrics", include_in_schema=False)

class StructuredLogger:
    def __init__(self, service_name: str):
        self.service_name = service_name
        self.logger = logging.getLogger(__name__)

    def _log(self, level: str, message: str, **kwargs):
        log_entry = {
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.%fZ", time.gmtime()),
            "level": level,
            "service": self.service_name,
            "message": message,
            **kwargs,
        }
        print(json.dumps(log_entry))

    def info(self, message: str, **kwargs):
        self._log("INFO", message, **kwargs)

    def error(self, message: str, **kwargs):
        self._log("ERROR", message, **kwargs)

    def warning(self, message: str, **kwargs):
        self._log("WARNING", message, **kwargs)

logger = StructuredLogger("books")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],            
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.exception_handler(RequestValidationError)
async def validation_exception_handler(_: Request, exc: RequestValidationError):
    return JSONResponse(
        status_code=422,
        content={"error": "Validation failed", "details": {"validation_errors": exc.errors()}},
    )

@app.exception_handler(HTTPException)
async def http_exception_handler(_: Request, exc: HTTPException):
    return JSONResponse(status_code=exc.status_code, content={"error": exc.detail, "details": {}})

@app.middleware("http")
async def add_request_id(request: Request, call_next):
    request_id = request.headers.get("X-Request-ID", str(uuid.uuid4()))
    start_time = time.time()
    logger.info("Request started", method=request.method, path=str(request.url.path), request_id=request_id)
    response = await call_next(request)
    latency = round((time.time() - start_time) * 1000, 2)
    logger.info("Request completed", method=request.method, path=str(request.url.path),
                status=response.status_code, latency=latency, request_id=request_id)
    response.headers["X-Request-ID"] = request_id
    return response

@app.post("/v1/books", response_model=BookResponse, status_code=201)
async def create_book(book_data: BookRequest, db: Session = Depends(get_db_session)):
    try:
        book = Book(title=book_data.title.strip(), author=book_data.author.strip(), price=book_data.price)
        db.add(book)
        db.commit()
        db.refresh(book)
        logger.info("Book created successfully", book_id=book.id, title=book.title)
        return BookResponse.model_validate(book)
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error creating book", error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/v1/books", response_model=List[BookResponse])
async def get_books(limit: int = 100, offset: int = 0, db: Session = Depends(get_db_session)):
    try:
        stmt = (
            select(Book)
            .where(Book.active.is_(True))
            .order_by(Book.created_at.desc())
            .limit(limit).offset(offset)
        )
        books = db.execute(stmt).scalars().all()
        return [BookResponse.model_validate(b) for b in books]
    except SQLAlchemyError as e:
        logger.error("Database error fetching books", error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/v1/books/{book_id}", response_model=BookResponse)
async def get_book(book_id: int, db: Session = Depends(get_db_session)):
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")
        return BookResponse.model_validate(book)
    except SQLAlchemyError as e:
        logger.error("Database error fetching book", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")

@app.put("/v1/books/{book_id}", response_model=BookResponse)
async def update_book(book_id: int, book_data: BookUpdateRequest, db: Session = Depends(get_db_session)):
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")
        if book_data.title is not None:
            book.title = book_data.title.strip()
        if book_data.author is not None:
            book.author = book_data.author.strip()
        if book_data.price is not None:
            book.price = book_data.price
        db.commit()
        db.refresh(book)
        logger.info("Book updated successfully", book_id=book.id, title=book.title)
        return BookResponse.model_validate(book)
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error updating book", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")

@app.delete("/v1/books/{book_id}")
async def delete_book(book_id: int, db: Session = Depends(get_db_session)):
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")
        book.active = False
        db.commit()
        logger.info("Book soft deleted successfully", book_id=book.id, title=book.title)
        return {"message": f"Book with ID {book.id} was deleted successfully"}
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error deleting book", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")

@app.get("/health")
async def health_check():
    try:
        from sqlalchemy import create_engine, text
        config = get_config()
        engine = create_engine(config["database_url"], pool_pre_ping=True)
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        return {"status": "healthy", "service": "books"}
    except Exception as e:
        logger.error("Health check failed", error=str(e))
        raise HTTPException(status_code=503, detail="Database connection failed")

if __name__ == "__main__":
    import uvicorn
    config = get_config()
    uvicorn.run("main:app", host="0.0.0.0", port=config["port"], reload=False, log_level="info")
