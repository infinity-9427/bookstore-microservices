from decimal import Decimal
from datetime import datetime
from sqlalchemy import BIGINT, NUMERIC, TEXT, TIMESTAMP, BOOLEAN, func, CheckConstraint
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class Book(Base):
    __tablename__ = "books"

    id: Mapped[int] = mapped_column(BIGINT, primary_key=True, autoincrement=True)
    title: Mapped[str] = mapped_column(TEXT, nullable=False)
    author: Mapped[str] = mapped_column(TEXT, nullable=False)
    price: Mapped[Decimal] = mapped_column(NUMERIC(10, 2), nullable=False)
    active: Mapped[bool] = mapped_column(BOOLEAN, nullable=False, server_default="true")
    created_at: Mapped[datetime] = mapped_column(
        TIMESTAMP(timezone=True), 
        nullable=False, 
        server_default=func.now()
    )
    updated_at: Mapped[datetime] = mapped_column(
        TIMESTAMP(timezone=True), 
        nullable=False, 
        server_default=func.now(),
        onupdate=func.now()
    )

    __table_args__ = (
        CheckConstraint("length(btrim(title)) > 0", name="check_title_not_empty"),
        CheckConstraint("length(btrim(author)) > 0", name="check_author_not_empty"),
        CheckConstraint("price >= 0", name="check_price_non_negative"),
    )