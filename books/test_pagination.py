"""
Test pagination functionality for Books API
"""
import pytest
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from decimal import Decimal

from main import app
from database import Base, Book
from db_session import get_db_session


# Create test database
SQLALCHEMY_DATABASE_URL = "sqlite:///./test.db"
engine = create_engine(SQLALCHEMY_DATABASE_URL, connect_args={"check_same_thread": False})
TestingSessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

def override_get_db():
    try:
        db = TestingSessionLocal()
        yield db
    finally:
        db.close()

app.dependency_overrides[get_db_session] = override_get_db

client = TestClient(app)


@pytest.fixture(scope="function")
def db_setup():
    """Create tables and sample data"""
    Base.metadata.create_all(bind=engine)
    
    # Add sample books
    db = TestingSessionLocal()
    try:
        for i in range(1, 51):  # 50 books
            book = Book(
                title=f"Book {i:02d}",
                author=f"Author {i:02d}",
                description=f"Description for book {i:02d}",
                price=Decimal(f"{10 + i}.99"),
                active=True,
            )
            db.add(book)
        db.commit()
        
        yield db
        
    finally:
        db.close()
        Base.metadata.drop_all(bind=engine)


def test_pagination_default_params(db_setup):
    """Test pagination with default parameters"""
    response = client.get("/v1/books")
    assert response.status_code == 200
    
    data = response.json()
    
    # Check response structure
    assert "data" in data
    assert "total" in data
    assert "limit" in data
    assert "offset" in data
    
    # Check pagination metadata
    assert data["total"] == 50
    assert data["limit"] == 20  # Default limit
    assert data["offset"] == 0   # Default offset
    assert len(data["data"]) == 20  # First page
    
    # Check headers
    assert response.headers.get("X-Total-Count") == "50"
    assert "Link" in response.headers
    assert 'rel="next"' in response.headers["Link"]
    assert 'rel="prev"' not in response.headers["Link"]  # First page


def test_pagination_custom_params(db_setup):
    """Test pagination with custom parameters"""
    response = client.get("/v1/books?limit=10&offset=20")
    assert response.status_code == 200
    
    data = response.json()
    
    # Check pagination metadata
    assert data["total"] == 50
    assert data["limit"] == 10
    assert data["offset"] == 20
    assert len(data["data"]) == 10
    
    # Check headers - should have both next and prev
    link_header = response.headers["Link"]
    assert 'rel="next"' in link_header
    assert 'rel="prev"' in link_header
    assert 'offset=30' in link_header  # Next page
    assert 'offset=10' in link_header  # Previous page


def test_pagination_last_page(db_setup):
    """Test pagination on last page"""
    response = client.get("/v1/books?limit=20&offset=40")
    assert response.status_code == 200
    
    data = response.json()
    
    # Check pagination metadata
    assert data["total"] == 50
    assert data["limit"] == 20
    assert data["offset"] == 40
    assert len(data["data"]) == 10  # Last 10 books
    
    # Check headers - should have prev but not next
    link_header = response.headers["Link"]
    assert 'rel="prev"' in link_header
    assert 'rel="next"' not in link_header
    assert 'offset=20' in link_header  # Previous page


def test_pagination_beyond_total(db_setup):
    """Test pagination beyond total records"""
    response = client.get("/v1/books?limit=20&offset=100")
    assert response.status_code == 200
    
    data = response.json()
    
    # Check pagination metadata
    assert data["total"] == 50
    assert data["limit"] == 20
    assert data["offset"] == 100
    assert len(data["data"]) == 0  # Empty array, not null
    assert data["data"] == []  # Explicitly empty array
    
    # Check headers - should have prev only
    link_header = response.headers["Link"]
    assert 'rel="prev"' in link_header
    assert 'rel="next"' not in link_header


def test_pagination_limit_validation(db_setup):
    """Test pagination limit validation"""
    # Test limit too low
    response = client.get("/v1/books?limit=0")
    assert response.status_code == 422
    
    # Test limit too high
    response = client.get("/v1/books?limit=200")
    assert response.status_code == 422
    
    # Test valid limit boundaries
    response = client.get("/v1/books?limit=1")
    assert response.status_code == 200
    
    response = client.get("/v1/books?limit=100")
    assert response.status_code == 200


def test_pagination_offset_validation(db_setup):
    """Test pagination offset validation"""
    # Test negative offset
    response = client.get("/v1/books?offset=-1")
    assert response.status_code == 422
    
    # Test valid offset
    response = client.get("/v1/books?offset=0")
    assert response.status_code == 200


def test_pagination_ordering(db_setup):
    """Test that pagination results are ordered by created_at DESC"""
    response = client.get("/v1/books?limit=5&offset=0")
    assert response.status_code == 200
    
    data = response.json()
    books = data["data"]
    
    # Check ordering - should be DESC by created_at (newest first)
    # In our test data, higher ID = newer (created later)
    assert len(books) == 5
    
    # Verify descending order by checking IDs
    ids = [book["id"] for book in books]
    assert ids == sorted(ids, reverse=True)


def test_data_never_null(db_setup):
    """Test that data array is never null, even when empty"""
    # Test with no results
    response = client.get("/v1/books?limit=10&offset=1000")
    assert response.status_code == 200
    
    data = response.json()
    assert data["data"] is not None
    assert isinstance(data["data"], list)
    assert data["data"] == []


def test_total_count_consistency(db_setup):
    """Test that total count is consistent across pages"""
    # First page
    response1 = client.get("/v1/books?limit=10&offset=0")
    assert response1.status_code == 200
    total1 = response1.json()["total"]
    
    # Second page
    response2 = client.get("/v1/books?limit=10&offset=10")
    assert response2.status_code == 200
    total2 = response2.json()["total"]
    
    # Last page
    response3 = client.get("/v1/books?limit=10&offset=40")
    assert response3.status_code == 200
    total3 = response3.json()["total"]
    
    # All should have same total
    assert total1 == total2 == total3 == 50


def test_link_header_format(db_setup):
    """Test RFC5988 Link header format"""
    response = client.get("/v1/books?limit=10&offset=20")
    assert response.status_code == 200
    
    link_header = response.headers["Link"]
    
    # Should contain properly formatted links
    assert "</v1/books?limit=10&offset=30>; rel=\"next\"" in link_header
    assert "</v1/books?limit=10&offset=10>; rel=\"prev\"" in link_header
    
    # Links should be comma-separated
    assert ", " in link_header


if __name__ == "__main__":
    pytest.main([__file__, "-v"])