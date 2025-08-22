import pytest
from decimal import Decimal
from pydantic import ValidationError

from models import BookRequest, BookUpdateRequest, BookResponse


class TestBookRequest:
    """Test suite for BookRequest model"""

    def test_valid_book_request(self):
        """Test valid book request creation"""
        book = BookRequest(
            title="Valid Book",
            author="Valid Author", 
            price=Decimal("19.99")
        )
        assert book.title == "Valid Book"
        assert book.author == "Valid Author"
        assert book.price == Decimal("19.99")

    def test_whitespace_stripping(self):
        """Test that whitespace is stripped"""
        book = BookRequest(
            title="  Test Book  ",
            author="  Test Author  ",
            price=Decimal("10.99")
        )
        assert book.title == "Test Book"
        assert book.author == "Test Author"

    def test_whitespace_normalization(self):
        """Test that internal whitespace is normalized"""
        book = BookRequest(
            title="Test    Book    Title",
            author="Test   Author   Name",
            price=Decimal("10.99")
        )
        assert book.title == "Test Book Title"
        assert book.author == "Test Author Name"

    def test_empty_title_validation(self):
        """Test that empty title is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="",
                author="Valid Author",
                price=Decimal("10.99")
            )

    def test_empty_author_validation(self):
        """Test that empty author is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="",
                price=Decimal("10.99")
            )

    def test_negative_price_validation(self):
        """Test that negative price is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="Valid Author",
                price=Decimal("-10.99")
            )

    def test_price_precision_validation(self):
        """Test price precision validation"""
        # Valid 2 decimal places
        book = BookRequest(
            title="Valid Title",
            author="Valid Author", 
            price=Decimal("19.99")
        )
        assert book.price == Decimal("19.99")

        # Invalid more than 2 decimal places
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="Valid Author",
                price=Decimal("19.999")
            )

    def test_price_rounding(self):
        """Test price rounding behavior"""
        book = BookRequest(
            title="Valid Title",
            author="Valid Author",
            price=Decimal("19.9")
        )
        assert book.price == Decimal("19.90")


class TestBookUpdateRequest:
    """Test suite for BookUpdateRequest model"""

    def test_valid_partial_update(self):
        """Test valid partial update"""
        update = BookUpdateRequest(title="New Title")
        assert update.title == "New Title"
        assert update.author is None
        assert update.price is None

    def test_valid_full_update(self):
        """Test valid full update"""
        update = BookUpdateRequest(
            title="New Title",
            author="New Author",
            price=Decimal("29.99")
        )
        assert update.title == "New Title"
        assert update.author == "New Author"
        assert update.price == Decimal("29.99")

    def test_empty_update_validation(self):
        """Test that empty update is rejected"""
        with pytest.raises(ValidationError):
            BookUpdateRequest()

    def test_whitespace_normalization(self):
        """Test whitespace normalization in updates"""
        update = BookUpdateRequest(
            title="  New   Title  ",
            author="  New   Author  "
        )
        assert update.title == "New Title"
        assert update.author == "New Author"

    def test_none_values_allowed(self):
        """Test that None values are allowed for individual fields"""
        update = BookUpdateRequest(
            title="New Title",
            author=None,
            price=None
        )
        assert update.title == "New Title"
        assert update.author is None
        assert update.price is None

    def test_price_validation_in_update(self):
        """Test price validation in updates"""
        # Valid price
        update = BookUpdateRequest(price=Decimal("15.99"))
        assert update.price == Decimal("15.99")

        # Invalid negative price
        with pytest.raises(ValidationError):
            BookUpdateRequest(price=Decimal("-15.99"))

        # Invalid precision
        with pytest.raises(ValidationError):
            BookUpdateRequest(price=Decimal("15.999"))


class TestBookResponse:
    """Test suite for BookResponse model"""

    def test_from_attributes_config(self):
        """Test that model can be created from SQLAlchemy objects"""
        # This would typically be tested with an actual SQLAlchemy object
        # but we can test the configuration is correct
        assert BookResponse.model_config["from_attributes"] is True

    def test_model_validation(self):
        """Test basic model validation"""
        from datetime import datetime
        
        # Create a mock object similar to SQLAlchemy result
        class MockBook:
            id = 1
            title = "Test Book"
            author = "Test Author"
            price = Decimal("19.99")
            active = True
            created_at = datetime.now()
            updated_at = datetime.now()

        response = BookResponse.model_validate(MockBook())
        assert response.id == 1
        assert response.title == "Test Book"
        assert response.author == "Test Author"
        assert response.price == Decimal("19.99")
        assert response.active is True