from decimal import Decimal, ROUND_HALF_UP
from datetime import datetime
from typing import Optional, List, Generic, TypeVar
import re

from typing_extensions import Annotated
from pydantic import (
    BaseModel,
    Field,
    field_validator,
    model_validator,
    StringConstraints,
    ConfigDict,
)

NonEmptyStr = Annotated[str, StringConstraints(min_length=1, strip_whitespace=True)]

T = TypeVar('T')


class ImageData(BaseModel):
    """Cloudinary image data"""
    url: str = Field(..., description="Cloudinary image URL")
    public_id: str = Field(..., description="Cloudinary public ID for image management")
    
    @field_validator('url')
    @classmethod
    def validate_url(cls, v: str) -> str:
        if not v.startswith(('http://', 'https://')):
            raise ValueError('URL must start with http:// or https://')
        return v


class BookRequest(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    title: NonEmptyStr = Field(..., description="Book title")
    author: NonEmptyStr = Field(..., description="Book author")
    description: NonEmptyStr = Field(..., description="Book description")
    price: Decimal = Field(..., ge=0, description="Book price (two decimals)")

    @field_validator("title", "author", "description")
    @classmethod
    def normalize_space(cls, v: str) -> str:
        # Collapse internal whitespace to a single space
        return re.sub(r"\s+", " ", v)

    @field_validator("price")
    @classmethod
    def validate_price_precision(cls, v: Decimal) -> Decimal:
        # Reject > 2 decimal places, then normalize to 2dp using bankers-safe rounding
        if v.as_tuple().exponent < -2:
            raise ValueError("Price cannot have more than 2 decimal places")
        return v.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)


class BookUpdateRequest(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)

    title: Optional[NonEmptyStr] = Field(None, description="Book title")
    author: Optional[NonEmptyStr] = Field(None, description="Book author")
    description: Optional[NonEmptyStr] = Field(None, description="Book description")
    price: Optional[Decimal] = Field(None, ge=0, description="Book price (two decimals)")

    @field_validator("title", "author", "description")
    @classmethod
    def normalize_space_optional(cls, v: Optional[str]) -> Optional[str]:
        if v is None:
            return v
        return re.sub(r"\s+", " ", v)

    @field_validator("price")
    @classmethod
    def validate_price_precision(cls, v: Optional[Decimal]) -> Optional[Decimal]:
        if v is None:
            return v
        if v.as_tuple().exponent < -2:
            raise ValueError("Price cannot have more than 2 decimal places")
        return v.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)

    @model_validator(mode="after")
    def at_least_one_field(self) -> "BookUpdateRequest":
        if self.title is None and self.author is None and self.description is None and self.price is None:
            raise ValueError("Provide at least one field to update")
        return self


class BookResponse(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: int
    title: str
    author: str
    description: str
    price: Decimal
    active: bool
    image: Optional[ImageData] = Field(None, description="Book cover image")
    created_at: datetime
    updated_at: datetime


class PaginatedResponse(BaseModel, Generic[T]):
    """Pagination wrapper with RFC5988 Link header support"""
    data: List[T] = Field(..., description="Page data (never null, can be empty)")
    total: int = Field(..., ge=0, description="Total records for the same filter set")
    limit: int = Field(..., ge=1, le=100, description="Page size returned")
    offset: int = Field(..., ge=0, description="Zero-based start index")


# Type alias for books pagination
BooksPage = PaginatedResponse[BookResponse]
