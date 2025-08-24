from decimal import Decimal
from datetime import datetime
from typing import Optional

from sqlalchemy import (
    BIGINT,
    NUMERIC,
    TEXT,
    TIMESTAMP,
    BOOLEAN,
    JSON,
    func,
    CheckConstraint,
)
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class Book(Base):
    __tablename__ = "books"
    # Fetch server defaults (e.g., now()) on insert/refresh automatically
    __mapper_args__ = {"eager_defaults": True}

    id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    title: Mapped[str] = mapped_column(TEXT, nullable=False)
    author: Mapped[str] = mapped_column(TEXT, nullable=False)
    description: Mapped[str] = mapped_column(TEXT, nullable=False)
    price: Mapped[Decimal] = mapped_column(NUMERIC(10, 2), nullable=False)
    active: Mapped[bool] = mapped_column(BOOLEAN, nullable=False, server_default="true")
    
    # Image field - stores Cloudinary URL and public_id as JSON
    image: Mapped[Optional[dict]] = mapped_column(JSON, nullable=True)

    created_at: Mapped[datetime] = mapped_column(
        TIMESTAMP(timezone=True),
        nullable=False,
        server_default=func.now(),
    )
    updated_at: Mapped[datetime] = mapped_column(
        TIMESTAMP(timezone=True),
        nullable=False,
        server_default=func.now(),
        onupdate=func.now(),  # aligns with DB trigger; harmless if both exist
    )

    __table_args__ = (
        CheckConstraint("length(trim(title)) > 0", name="check_title_not_empty"),
        CheckConstraint("length(trim(author)) > 0", name="check_author_not_empty"),
        CheckConstraint("length(trim(description)) > 0", name="check_description_not_empty"),
        CheckConstraint("price >= 0", name="check_price_non_negative"),
    )
