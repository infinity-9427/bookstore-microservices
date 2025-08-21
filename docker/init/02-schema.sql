-- Schema for books_db
\c books_db

CREATE TABLE books (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    author TEXT NOT NULL CHECK (length(btrim(author)) > 0),
    price NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Create indexes for better query performance
CREATE INDEX idx_books_title ON books(title);
CREATE INDEX idx_books_author ON books(author);
CREATE INDEX idx_books_active ON books(active);

-- Create trigger function for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for books table
CREATE TRIGGER update_books_updated_at
    BEFORE UPDATE ON books
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Schema for orders_db
\c orders_db

CREATE TABLE orders (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    book_id BIGINT NOT NULL,
    book_title TEXT NOT NULL CHECK (length(btrim(book_title)) > 0),
    book_author TEXT NOT NULL CHECK (length(btrim(book_author)) > 0),
    quantity INT NOT NULL CHECK (quantity > 0 AND quantity <= 100000),
    unit_price NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(12,2) GENERATED ALWAYS AS ((quantity::numeric * unit_price)::numeric(12,2)) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Create indexes for orders
CREATE INDEX idx_orders_book_id ON orders(book_id);
CREATE INDEX idx_orders_created_at ON orders(created_at);