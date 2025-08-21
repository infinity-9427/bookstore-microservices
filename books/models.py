from decimal import Decimal
from datetime import datetime
from typing import Optional
from pydantic import BaseModel, Field, field_validator


class BookRequest(BaseModel):
    title: str = Field(..., min_length=1, description="Book title")
    author: str = Field(..., min_length=1, description="Book author")
    price: Decimal = Field(..., ge=0, description="Book price")

    @field_validator('title', 'author')
    @classmethod
    def validate_trimmed_nonblank(cls, v: str) -> str:
        trimmed = v.strip()
        if not trimmed:
            raise ValueError("Field cannot be empty or whitespace only")
        return trimmed

    @field_validator('price')
    @classmethod
    def validate_price_precision(cls, v: Decimal) -> Decimal:
        if v.as_tuple().exponent < -2:
            raise ValueError("Price cannot have more than 2 decimal places")
        return v


class BookResponse(BaseModel):
    id: int
    title: str
    author: str
    price: Decimal
    active: bool
    created_at: datetime
    updated_at: datetime

    class Config:
        from_attributes = True