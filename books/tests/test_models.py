import pytest
from decimal import Decimal
from pydantic import ValidationError

from models import BookRequest, BookUpdateRequest, BookResponse, ImageData


class TestImageData:
    """Test suite for ImageData model"""

    def test_valid_image_data(self):
        """Test valid image data creation"""
        image = ImageData(
            url="https://cloudinary.com/image123.jpg",
            public_id="image123"
        )
        assert image.url == "https://cloudinary.com/image123.jpg"
        assert image.public_id == "image123"

    def test_http_url_validation(self):
        """Test URL validation for http URLs"""
        image = ImageData(
            url="http://example.com/image.jpg",
            public_id="test123"
        )
        assert image.url == "http://example.com/image.jpg"

    def test_invalid_url_validation(self):
        """Test that invalid URLs are rejected"""
        with pytest.raises(ValidationError):
            ImageData(
                url="invalid-url",
                public_id="test123"
            )

    def test_ftp_url_rejected(self):
        """Test that non-HTTP URLs are rejected"""
        with pytest.raises(ValidationError):
            ImageData(
                url="ftp://example.com/image.jpg",
                public_id="test123"
            )


class TestBookRequest:
    """Test suite for BookRequest model"""

    def test_valid_book_request(self):
        """Test valid book request creation"""
        book = BookRequest(
            title="Valid Book",
            author="Valid Author",
            description="A captivating story about adventure and discovery",
            price=Decimal("19.99")
        )
        assert book.title == "Valid Book"
        assert book.author == "Valid Author"
        assert book.description == "A captivating story about adventure and discovery"
        assert book.price == Decimal("19.99")

    def test_whitespace_stripping(self):
        """Test that whitespace is stripped"""
        book = BookRequest(
            title="  Test Book  ",
            author="  Test Author  ",
            description="  A fascinating tale  ",
            price=Decimal("10.99")
        )
        assert book.title == "Test Book"
        assert book.author == "Test Author"
        assert book.description == "A fascinating tale"

    def test_whitespace_normalization(self):
        """Test that internal whitespace is normalized"""
        book = BookRequest(
            title="Test    Book    Title",
            author="Test   Author   Name",
            description="A   thrilling    adventure    story",
            price=Decimal("10.99")
        )
        assert book.title == "Test Book Title"
        assert book.author == "Test Author Name"
        assert book.description == "A thrilling adventure story"

    def test_empty_title_validation(self):
        """Test that empty title is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="",
                author="Valid Author",
                description="Valid description",
                price=Decimal("10.99")
            )

    def test_empty_author_validation(self):
        """Test that empty author is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="",
                description="Valid description",
                price=Decimal("10.99")
            )
    
    def test_empty_description_validation(self):
        """Test that empty description is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="Valid Author",
                description="",
                price=Decimal("10.99")
            )

    def test_negative_price_validation(self):
        """Test that negative price is rejected"""
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="Valid Author",
                description="Valid description",
                price=Decimal("-10.99")
            )

    def test_price_precision_validation(self):
        """Test price precision validation"""
        # Valid 2 decimal places
        book = BookRequest(
            title="Valid Title",
            author="Valid Author",
            description="Valid description",
            price=Decimal("19.99")
        )
        assert book.price == Decimal("19.99")

        # Invalid more than 2 decimal places
        with pytest.raises(ValidationError):
            BookRequest(
                title="Valid Title",
                author="Valid Author",
                description="Valid description",
                price=Decimal("19.999")
            )

    def test_price_rounding(self):
        """Test price rounding behavior"""
        book = BookRequest(
            title="Valid Title",
            author="Valid Author",
            description="Valid description",
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
        assert update.description is None
        assert update.price is None

    def test_valid_full_update(self):
        """Test valid full update"""
        update = BookUpdateRequest(
            title="New Title",
            author="New Author",
            description="An updated description",
            price=Decimal("29.99")
        )
        assert update.title == "New Title"
        assert update.author == "New Author"
        assert update.description == "An updated description"
        assert update.price == Decimal("29.99")

    def test_empty_update_validation(self):
        """Test that empty update is rejected"""
        with pytest.raises(ValidationError):
            BookUpdateRequest()

    def test_whitespace_normalization(self):
        """Test whitespace normalization in updates"""
        update = BookUpdateRequest(
            title="  New   Title  ",
            author="  New   Author  ",
            description="  Updated   description  "
        )
        assert update.title == "New Title"
        assert update.author == "New Author"
        assert update.description == "Updated description"

    def test_none_values_allowed(self):
        """Test that None values are allowed for individual fields"""
        update = BookUpdateRequest(
            title="New Title",
            author=None,
            description=None,
            price=None
        )
        assert update.title == "New Title"
        assert update.author is None
        assert update.description is None
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
            description = "A fascinating test book"
            price = Decimal("19.99")
            active = True
            image = None
            created_at = datetime.now()
            updated_at = datetime.now()

        response = BookResponse.model_validate(MockBook())
        assert response.id == 1
        assert response.title == "Test Book"
        assert response.author == "Test Author"
        assert response.description == "A fascinating test book"
        assert response.price == Decimal("19.99")
        assert response.active is True
        assert response.image is None

    def test_model_validation_with_image(self):
        """Test model validation with image data"""
        from datetime import datetime
        
        # Create a mock object with image data
        class MockBookWithImage:
            id = 2
            title = "Book with Image"
            author = "Image Author"
            description = "A book with beautiful cover art"
            price = Decimal("25.99")
            active = True
            image = {
                "url": "https://cloudinary.com/book-cover.jpg",
                "public_id": "book-cover-123"
            }
            created_at = datetime.now()
            updated_at = datetime.now()

        response = BookResponse.model_validate(MockBookWithImage())
        assert response.id == 2
        assert response.title == "Book with Image"
        assert response.description == "A book with beautiful cover art"
        assert response.image is not None
        assert response.image.url == "https://cloudinary.com/book-cover.jpg"
        assert response.image.public_id == "book-cover-123"