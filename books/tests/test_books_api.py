import pytest
from decimal import Decimal
from fastapi.testclient import TestClient
from httpx import AsyncClient

from database import Book


class TestBooksAPI:
    """Test suite for Books API endpoints"""

    def test_health_endpoint(self, client: TestClient):
        """Test health check endpoint"""
        response = client.get("/health")
        assert response.status_code == 200
        assert response.json() == {"status": "healthy", "service": "books"}

    def test_create_book_success(self, client: TestClient):
        """Test successful book creation"""
        book_data = {
            "title": "The Great Gatsby",
            "author": "F. Scott Fitzgerald",
            "description": "A classic American novel set in the Jazz Age",
            "price": "19.99"
        }
        response = client.post("/v1/books", json=book_data)
        assert response.status_code == 201
        
        data = response.json()
        assert data["title"] == "The Great Gatsby"
        assert data["author"] == "F. Scott Fitzgerald"
        assert data["description"] == "A classic American novel set in the Jazz Age"
        assert Decimal(data["price"]) == Decimal("19.99")
        assert data["active"] is True
        assert data["image"] is None  # Image should be None by default
        assert "id" in data
        assert "created_at" in data
        assert "updated_at" in data

    def test_create_book_invalid_data(self, client: TestClient):
        """Test book creation with invalid data"""
        invalid_data = {
            "title": "",  # Empty title
            "author": "Author Name",
            "description": "Valid description",
            "price": "19.99"
        }
        response = client.post("/v1/books", json=invalid_data)
        assert response.status_code == 422

    def test_create_book_negative_price(self, client: TestClient):
        """Test book creation with negative price"""
        invalid_data = {
            "title": "Test Book",
            "author": "Test Author",
            "price": "-10.99"
        }
        response = client.post("/v1/books", json=invalid_data)
        assert response.status_code == 422

    def test_create_book_precision_validation(self, client: TestClient):
        """Test price precision validation"""
        invalid_data = {
            "title": "Test Book",
            "author": "Test Author", 
            "price": "19.999"  # More than 2 decimal places
        }
        response = client.post("/v1/books", json=invalid_data)
        assert response.status_code == 422

    def test_get_books_empty(self, client: TestClient):
        """Test getting books when none exist"""
        response = client.get("/v1/books")
        assert response.status_code == 200
        assert response.json() == []

    def test_get_books_with_data(self, client: TestClient, test_db_session):
        """Test getting books when data exists"""
        # Create test books directly in database
        book1 = Book(title="Book 1", author="Author 1", description="Description 1", price=Decimal("10.99"))
        book2 = Book(title="Book 2", author="Author 2", description="Description 2", price=Decimal("15.99"))
        test_db_session.add_all([book1, book2])
        test_db_session.commit()
        
        response = client.get("/v1/books")
        assert response.status_code == 200
        
        data = response.json()
        assert len(data) == 2
        assert data[0]["title"] in ["Book 1", "Book 2"]
        assert data[1]["title"] in ["Book 1", "Book 2"]

    def test_get_books_pagination(self, client: TestClient, test_db_session):
        """Test books pagination"""
        # Create multiple test books
        books = [Book(title=f"Book {i}", author=f"Author {i}", description=f"Description {i}", price=Decimal("10.99")) 
                for i in range(5)]
        test_db_session.add_all(books)
        test_db_session.commit()
        
        # Test limit
        response = client.get("/v1/books?limit=3")
        assert response.status_code == 200
        assert len(response.json()) == 3
        
        # Test offset
        response = client.get("/v1/books?limit=2&offset=2")
        assert response.status_code == 200
        assert len(response.json()) == 2

    def test_get_book_by_id_success(self, client: TestClient, test_db_session):
        """Test getting a specific book by ID"""
        book = Book(title="Test Book", author="Test Author", description="Test description", price=Decimal("20.99"))
        test_db_session.add(book)
        test_db_session.commit()
        test_db_session.refresh(book)
        
        response = client.get(f"/v1/books/{book.id}")
        assert response.status_code == 200
        
        data = response.json()
        assert data["id"] == book.id
        assert data["title"] == "Test Book"
        assert data["author"] == "Test Author"
        assert Decimal(data["price"]) == Decimal("20.99")
        assert data["image"] is None

    def test_get_book_by_id_not_found(self, client: TestClient):
        """Test getting a non-existent book"""
        response = client.get("/v1/books/999999")
        assert response.status_code == 404
        assert response.json()["error"] == "Book not found"

    def test_update_book_success(self, client: TestClient, test_db_session):
        """Test successful book update"""
        book = Book(title="Old Title", author="Old Author", price=Decimal("10.99"))
        test_db_session.add(book)
        test_db_session.commit()
        test_db_session.refresh(book)
        
        update_data = {
            "title": "New Title",
            "price": "25.99"
        }
        response = client.put(f"/v1/books/{book.id}", json=update_data)
        assert response.status_code == 200
        
        data = response.json()
        assert data["title"] == "New Title"
        assert data["author"] == "Old Author"  # Unchanged
        assert Decimal(data["price"]) == Decimal("25.99")

    def test_update_book_not_found(self, client: TestClient):
        """Test updating a non-existent book"""
        update_data = {"title": "New Title"}
        response = client.put("/v1/books/999999", json=update_data)
        assert response.status_code == 404

    def test_update_book_no_fields(self, client: TestClient, test_db_session):
        """Test updating book with no fields provided"""
        book = Book(title="Test Book", author="Test Author", price=Decimal("10.99"))
        test_db_session.add(book)
        test_db_session.commit()
        test_db_session.refresh(book)
        
        response = client.put(f"/v1/books/{book.id}", json={})
        assert response.status_code == 422

    def test_delete_book_success(self, client: TestClient, test_db_session):
        """Test successful book deletion (soft delete)"""
        book = Book(title="Book to Delete", author="Author", price=Decimal("10.99"))
        test_db_session.add(book)
        test_db_session.commit()
        test_db_session.refresh(book)
        
        response = client.delete(f"/v1/books/{book.id}")
        assert response.status_code == 200
        assert response.json()["message"] == f"Book with ID {book.id} was deleted successfully"
        
        # Verify book is soft deleted
        test_db_session.refresh(book)
        assert book.active is False
        
        # Verify book doesn't appear in get requests
        response = client.get(f"/v1/books/{book.id}")
        assert response.status_code == 404

    def test_delete_book_not_found(self, client: TestClient):
        """Test deleting a non-existent book"""
        response = client.delete("/v1/books/999999")
        assert response.status_code == 404

    def test_whitespace_normalization(self, client: TestClient):
        """Test that whitespace is properly normalized"""
        book_data = {
            "title": "  The   Great    Gatsby  ",
            "author": "   F.  Scott   Fitzgerald   ",
            "price": "19.99"
        }
        response = client.post("/v1/books", json=book_data)
        assert response.status_code == 201
        
        data = response.json()
        assert data["title"] == "The Great Gatsby"
        assert data["author"] == "F. Scott Fitzgerald"

    def test_create_book_json_missing_title(self, client: TestClient):
        """Test JSON request missing title returns 422"""
        json_data = {
            "author": "Valid Author",
            "description": "Valid description", 
            "price": "19.99"
            # Missing title
        }
        response = client.post("/v1/books", json=json_data)
        assert response.status_code == 422

    def test_create_book_multipart_missing_title(self, client: TestClient):
        """Test multipart request missing title returns 422"""
        # Create multipart request by including an actual file entry
        from io import BytesIO
        response = client.post(
            "/v1/books",
            data={
                "author": "Valid Author",
                "description": "Valid description", 
                "price": "19.99"
                # Missing title
            },
            files={"dummy": ("", BytesIO(b""), "text/plain")}  # Dummy file to trigger multipart
        )
        assert response.status_code == 422

    def test_openapi_multipart_schema_required_fields(self, client: TestClient):
        """Test that OpenAPI schema shows correct required fields for multipart"""
        response = client.get("/openapi.json")
        assert response.status_code == 200
        
        openapi = response.json()
        
        # Check that the multipart schema has the correct required fields
        create_book_path = openapi["paths"]["/v1/books"]["post"]
        multipart_schema = create_book_path["requestBody"]["content"]["multipart/form-data"]["schema"]
        
        assert "required" in multipart_schema
        assert set(multipart_schema["required"]) == {"title", "author", "description", "price"}
        
        # Verify that required fields are not nullable in properties
        properties = multipart_schema["properties"]
        for field in ["title", "author", "description", "price"]:
            assert field in properties
            # Should not have nullable: true
            assert properties[field].get("nullable") != True
        
        # Image should be optional (not in required list)
        assert "image" not in multipart_schema["required"]


@pytest.mark.asyncio
class TestBooksAPIAsync:
    """Async test suite for Books API"""

    async def test_create_book_async(self, async_client: AsyncClient):
        """Test async book creation"""
        book_data = {
            "title": "Async Book",
            "author": "Async Author",
            "price": "29.99"
        }
        response = await async_client.post("/v1/books", json=book_data)
        assert response.status_code == 201
        
        data = response.json()
        assert data["title"] == "Async Book"
        assert data["author"] == "Async Author"

    async def test_get_books_async(self, async_client: AsyncClient):
        """Test async get books"""
        response = await async_client.get("/v1/books")
        assert response.status_code == 200
        assert isinstance(response.json(), list)