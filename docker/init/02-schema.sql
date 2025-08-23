-----------------------------------------------------------------------
-- Schema for books_db
-----------------------------------------------------------------------
\connect books_db

-- Books table with required description and optional image field
CREATE TABLE IF NOT EXISTS books (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    author TEXT NOT NULL CHECK (length(btrim(author)) > 0),
    description TEXT NOT NULL CHECK (length(btrim(description)) > 0),
    price NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    image JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- updated_at trigger for books
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_books_set_updated_at ON books;
CREATE TRIGGER trg_books_set_updated_at
BEFORE UPDATE ON books
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Indexes for books
CREATE INDEX IF NOT EXISTS idx_books_title ON books(title);
CREATE INDEX IF NOT EXISTS idx_books_author ON books(author);
CREATE INDEX IF NOT EXISTS idx_books_active_true ON books(active) WHERE active;
CREATE INDEX IF NOT EXISTS idx_books_created_at ON books(created_at DESC);

-----------------------------------------------------------------------
-- Schema for orders_db (with decimal arithmetic and idempotency support)
-----------------------------------------------------------------------
\connect orders_db

-- Orders table with exact decimal total_price
CREATE TABLE IF NOT EXISTS orders (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    total_price NUMERIC(12,2) NOT NULL CHECK (total_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Order items table with exact decimal arithmetic
CREATE TABLE IF NOT EXISTS order_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    book_id BIGINT NOT NULL,
    book_title TEXT NOT NULL,
    book_author TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(10,2) NOT NULL CHECK (total_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotency support for exactly-once semantics
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for orders
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_total_price ON orders(total_price);

-- Indexes for order_items
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_book_id ON order_items(book_id);
CREATE INDEX IF NOT EXISTS idx_order_items_created_at ON order_items(created_at);
CREATE INDEX IF NOT EXISTS idx_order_items_total_price ON order_items(total_price);

-- Indexes for idempotency_keys
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_order_id ON idempotency_keys(order_id);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created_at ON idempotency_keys(created_at);

-- Trigger to automatically sync order total_price when items change
CREATE OR REPLACE FUNCTION sync_order_total()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE orders 
    SET total_price = (
        SELECT COALESCE(SUM(oi.total_price), 0.00)
        FROM order_items oi 
        WHERE oi.order_id = COALESCE(NEW.order_id, OLD.order_id)
    )
    WHERE id = COALESCE(NEW.order_id, OLD.order_id);
    
    RETURN COALESCE(NEW, OLD);
END; $$ LANGUAGE plpgsql;

-- Triggers to maintain order total consistency
DROP TRIGGER IF EXISTS trg_sync_order_total_insert ON order_items;
CREATE TRIGGER trg_sync_order_total_insert
    AFTER INSERT ON order_items
    FOR EACH ROW
    EXECUTE FUNCTION sync_order_total();

DROP TRIGGER IF EXISTS trg_sync_order_total_update ON order_items;
CREATE TRIGGER trg_sync_order_total_update
    AFTER UPDATE ON order_items
    FOR EACH ROW
    EXECUTE FUNCTION sync_order_total();

DROP TRIGGER IF EXISTS trg_sync_order_total_delete ON order_items;
CREATE TRIGGER trg_sync_order_total_delete
    AFTER DELETE ON order_items
    FOR EACH ROW
    EXECUTE FUNCTION sync_order_total();

-- Comments documenting decimal arithmetic requirements
COMMENT ON COLUMN order_items.total_price IS 'Exact decimal total: quantity * unit_price, computed with NUMERIC precision';
COMMENT ON COLUMN order_items.unit_price IS 'Exact decimal price from Books API, stored as NUMERIC for precision';
COMMENT ON COLUMN orders.total_price IS 'Exact decimal total: sum of all item total_prices, maintained by triggers';
COMMENT ON TABLE idempotency_keys IS 'SHA-256 request body hashes for exactly-once order creation semantics';