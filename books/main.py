import logging
import uuid
import json
import time
from typing import List, Optional
from decimal import Decimal

from fastapi import FastAPI, HTTPException, Depends, Request, UploadFile, File, Form, Query, Response, Body
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from fastapi.exceptions import RequestValidationError

from sqlalchemy.orm import Session
from sqlalchemy import select, func
from sqlalchemy.exc import SQLAlchemyError

from models import BookRequest, BookResponse, BookUpdateRequest, ImageData, BooksPage
from database import Book
from db_session import get_db_session
from config import Config

from prometheus_fastapi_instrumentator import Instrumentator
from utils.cloudinary_utils import (
    init_cloudinary_if_configured,
    upload_book_cover,
    delete_cloudinary_image,
)

app = FastAPI(title="Books API", version="1.0.0")

# Initialize Cloudinary once (if configured and available)
CLOUDINARY_READY = init_cloudinary_if_configured()

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


async def upload_image_to_cloudinary(file: UploadFile) -> ImageData:
    """
    Validate the file (type/size) and upload via utils.upload_book_cover.
    Raises HTTPException on validation or upload failures.
    """
    if not CLOUDINARY_READY:
        raise HTTPException(status_code=501, detail="Image upload not configured")

    # Validate content type
    if file.content_type not in Config.ALLOWED_IMAGE_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid image type. Allowed types: {', '.join(Config.ALLOWED_IMAGE_TYPES)}",
        )

    # Validate size
    content = await file.read()
    await file.seek(0)  # reset for any later reads
    if len(content) > Config.MAX_IMAGE_SIZE:
        raise HTTPException(
            status_code=400,
            detail=f"Image size too large. Maximum size: {Config.MAX_IMAGE_SIZE / 1024 / 1024:.1f}MB",
        )

    try:
        return upload_book_cover(content)
    except Exception as e:
        logger.error("Cloudinary upload failed", error=str(e))
        raise HTTPException(status_code=500, detail="Failed to upload image")


async def delete_image_from_cloudinary(public_id: str) -> bool:
    """Best-effort deletion using utils.delete_cloudinary_image."""
    try:
        return delete_cloudinary_image(public_id)
    except Exception as e:
        logger.warning("Failed to delete image from Cloudinary", public_id=public_id, error=str(e))
        return False


app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # tighten in prod
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
    logger.info(
        "Request completed",
        method=request.method,
        path=str(request.url.path),
        status=response.status_code,
        latency=latency,
        request_id=request_id,
    )
    response.headers["X-Request-ID"] = request_id
    return response


# -------- CREATE (accepts JSON or multipart form-data) --------
@app.post(
    "/v1/books",
    response_model=BookResponse,
    status_code=201,
    openapi_extra={
        "requestBody": {
            "content": {
                "application/json": {
                    "schema": {"$ref": "#/components/schemas/BookRequest"}
                },
                "multipart/form-data": {
                    "schema": {
                        "type": "object",
                        "required": ["title", "author", "description", "price"],
                        "properties": {
                            "title": {"type": "string", "description": "Book title"},
                            "author": {"type": "string", "description": "Book author"},
                            "description": {"type": "string", "description": "Book description"},
                            "price": {"type": "number", "description": "Book price (two decimals)"},
                            "image": {"type": "string", "format": "binary", "description": "Optional book cover image"}
                        }
                    }
                }
            }
        }
    }
)
async def create_book(
    request: Request,
    # Optional form-data fields (ignored on JSON requests)
    title: Optional[str] = Form(None),
    author: Optional[str] = Form(None),
    description: Optional[str] = Form(None),
    price: Optional[float] = Form(None),
    image: Optional[UploadFile] = File(None),
    db: Session = Depends(get_db_session),
):
    try:
        content_type = (request.headers.get("content-type") or "").lower()

        # --- JSON path ---
        if "application/json" in content_type:
            payload = await request.json()
            from decimal import Decimal

            # Validate with Pydantic
            book_data = BookRequest(
                title=payload.get("title"),
                author=payload.get("author"),
                description=payload.get("description"),
                price=Decimal(str(payload.get("price"))),
            )

            # No file via JSON; users can call /{id}/image later
            image_data = None

        # --- multipart/form-data path ---
        elif "multipart/form-data" in content_type:
            # Ensure required form fields exist
            missing = [k for k, v in {"title": title, "author": author, "description": description, "price": price}.items() if v in (None, "")]
            if missing:
                raise HTTPException(status_code=422, detail=f"Missing required fields: {', '.join(missing)}")

            from decimal import Decimal
            book_data = BookRequest(
                title=title, author=author, description=description, price=Decimal(str(price))
            )

            image_data = None
            if image is not None:
                image_data = await upload_image_to_cloudinary(image)

        else:
            raise HTTPException(status_code=415, detail="Unsupported Media Type. Use application/json or multipart/form-data.")

        # Persist
        book = Book(
            title=book_data.title.strip(),
            author=book_data.author.strip(),
            description=book_data.description.strip(),
            price=book_data.price,
            image=image_data.model_dump() if image_data else None,
        )
        db.add(book)
        db.commit()
        db.refresh(book)
        logger.info("Book created successfully", book_id=book.id, title=book.title)

        response = BookResponse.model_validate(book)
        if book.image:
            response.image = ImageData(**book.image)
        return response

    except ValueError as e:
        logger.warning("Validation error creating book", error=str(e))
        raise HTTPException(status_code=422, detail=str(e))
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error creating book", error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")


@app.get("/v1/books", response_model=BooksPage)
async def get_books(
    response: Response,
    limit: int = Query(20, ge=1, le=100, description="Page size (1-100)"),
    offset: int = Query(0, ge=0, description="Zero-based start index"),
    db: Session = Depends(get_db_session)
):
    try:
        # Get total count with same filters
        count_stmt = select(func.count()).select_from(Book).where(Book.active.is_(True))
        total = db.execute(count_stmt).scalar()

        # Get page data
        stmt = (
            select(Book)
            .where(Book.active.is_(True))
            .order_by(Book.created_at.desc())
            .limit(limit)
            .offset(offset)
        )
        books = db.execute(stmt).scalars().all()

        # Build response objects
        responses: List[BookResponse] = []
        for book in books:
            book_response = BookResponse.model_validate(book)
            if book.image:
                book_response.image = ImageData(**book.image)
            responses.append(book_response)

        # Set pagination headers
        base_url = "/v1/books"
        
        # RFC5988 Link header
        links = []
        if offset + limit < total:
            next_offset = offset + limit
            links.append(f'<{base_url}?limit={limit}&offset={next_offset}>; rel="next"')
        if offset > 0:
            prev_offset = max(0, offset - limit)
            links.append(f'<{base_url}?limit={limit}&offset={prev_offset}>; rel="prev"')
        
        if links:
            response.headers["Link"] = ", ".join(links)
        
        # X-Total-Count header for simple client use
        response.headers["X-Total-Count"] = str(total)

        return BooksPage(
            data=responses,
            total=total,
            limit=limit,
            offset=offset
        )
    except SQLAlchemyError as e:
        logger.error("Database error fetching books", error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")


@app.get("/v1/books/{book_id}", response_model=BookResponse)
async def get_book(book_id: int, db: Session = Depends(get_db_session)):
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")

        response = BookResponse.model_validate(book)
        if book.image:
            response.image = ImageData(**book.image)
        return response
    except SQLAlchemyError as e:
        logger.error("Database error fetching book", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")


# -------- UPDATE (accepts JSON or multipart form-data) --------
@app.put("/v1/books/{book_id}", response_model=BookResponse)
async def update_book(
    book_id: int,
    request: Request,
    # optional form-data fields
    title: Optional[str] = Form(None),
    author: Optional[str] = Form(None),
    description: Optional[str] = Form(None),
    price: Optional[float] = Form(None),
    image: Optional[UploadFile] = File(None),
    remove_image: bool = Form(False),
    db: Session = Depends(get_db_session),
):
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")

        content_type = (request.headers.get("content-type") or "").lower()

        # --- JSON path ---
        if "application/json" in content_type:
            payload = await request.json()
            from decimal import Decimal

            # Accept any subset (including description)
            book_data = BookUpdateRequest(
                title=payload.get("title"),
                author=payload.get("author"),
                description=payload.get("description"),
                price=Decimal(str(payload["price"])) if "price" in payload and payload["price"] is not None else None,
            )
            json_remove = payload.get("remove_image", False)

            if book_data.title is not None:
                book.title = book_data.title.strip()
            if book_data.author is not None:
                book.author = book_data.author.strip()
            if book_data.description is not None:
                book.description = book_data.description.strip()
            if book_data.price is not None:
                book.price = book_data.price
            if json_remove and book.image:
                old = ImageData(**book.image)
                await delete_image_from_cloudinary(old.public_id)
                book.image = None

        # --- multipart/form-data path ---
        elif "multipart/form-data" in content_type:
            # Only update fields that are provided
            from decimal import Decimal

            # Apply partial updates
            if title is not None:
                book.title = BookUpdateRequest(title=title).title  # validator trims
            if author is not None:
                book.author = BookUpdateRequest(author=author).author
            if description is not None:
                book.description = BookUpdateRequest(description=description).description
            if price is not None:
                validated = BookUpdateRequest(price=Decimal(str(price))).price
                if validated is not None:
                    book.price = validated

            # Image handling
            if remove_image and book.image:
                old = ImageData(**book.image)
                await delete_image_from_cloudinary(old.public_id)
                book.image = None
            elif image is not None:
                # replace existing
                if book.image:
                    old = ImageData(**book.image)
                    await delete_image_from_cloudinary(old.public_id)
                uploaded = await upload_image_to_cloudinary(image)
                book.image = uploaded.model_dump()

        else:
            raise HTTPException(status_code=415, detail="Unsupported Media Type. Use application/json or multipart/form-data.")

        db.commit()
        db.refresh(book)
        logger.info("Book updated successfully", book_id=book.id, title=book.title)

        response = BookResponse.model_validate(book)
        if book.image:
            response.image = ImageData(**book.image)
        return response

    except ValueError as e:
        logger.warning("Validation error updating book", book_id=book_id, error=str(e))
        raise HTTPException(status_code=422, detail=str(e))
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

        # Delete image from Cloudinary if exists (best effort)
        if book.image:
            image_data = ImageData(**book.image)
            await delete_image_from_cloudinary(image_data.public_id)

        book.active = False
        db.commit()
        logger.info("Book soft deleted successfully", book_id=book.id, title=book.title)
        return {"message": f"Book with ID {book.id} was deleted successfully"}
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error deleting book", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")


@app.post("/v1/books/{book_id}/image", response_model=ImageData)
async def upload_book_image(book_id: int, image: UploadFile = File(...), db: Session = Depends(get_db_session)):
    """Upload/replace image for a specific book (still supported)."""
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")

        if book.image:
            old = ImageData(**book.image)
            await delete_image_from_cloudinary(old.public_id)

        uploaded = await upload_image_to_cloudinary(image)
        book.image = uploaded.model_dump()
        db.commit()
        db.refresh(book)
        logger.info("Book image uploaded successfully", book_id=book.id, title=book.title)
        return uploaded
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error uploading book image", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")


@app.delete("/v1/books/{book_id}/image")
async def delete_book_image(book_id: int, db: Session = Depends(get_db_session)):
    """Delete image for a specific book."""
    try:
        book = db.execute(select(Book).where(Book.id == book_id, Book.active.is_(True))).scalar_one_or_none()
        if not book:
            raise HTTPException(status_code=404, detail="Book not found")

        if not book.image:
            raise HTTPException(status_code=404, detail="Book has no image")

        image_data = ImageData(**book.image)
        deleted = await delete_image_from_cloudinary(image_data.public_id)
        book.image = None
        db.commit()
        logger.info("Book image deleted successfully", book_id=book.id, title=book.title)
        return {"message": "Book image deleted successfully", "cloudinary_deleted": deleted}
    except SQLAlchemyError as e:
        db.rollback()
        logger.error("Database error deleting book image", book_id=book_id, error=str(e))
        raise HTTPException(status_code=500, detail="Internal server error")


@app.get("/health")
async def health_check():
    try:
        from sqlalchemy import create_engine, text
        engine = create_engine(Config.BOOKS_DB_DSN, pool_pre_ping=True)
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        return {
            "status": "healthy",
            "service": "books",
            "cloudinary_configured": Config.has_cloudinary(),
            "cloudinary_ready": CLOUDINARY_READY,
        }
    except Exception as e:
        logger.error("Health check failed", error=str(e))
        raise HTTPException(status_code=503, detail="Database connection failed")


if __name__ == "__main__":
    import uvicorn
    Config.validate_required()
    uvicorn.run("main:app", host="0.0.0.0", port=Config.PORT, reload=False, log_level="info")
