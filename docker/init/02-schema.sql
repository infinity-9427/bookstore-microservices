-----------------------------------------------------------------------
-- Schema for books_db
-----------------------------------------------------------------------
\connect books_db

CREATE TABLE books (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title      TEXT NOT NULL CHECK (length(btrim(title))  > 0),
    author     TEXT NOT NULL CHECK (length(btrim(author)) > 0),
    price      NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- updated_at trigger
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

-- OPTIONAL: enforce immutability of core fields (title/author/price)
-- Comment this block out if you want to allow edits.
-- CREATE OR REPLACE FUNCTION books_immutable_fields()
-- RETURNS TRIGGER AS $$
-- BEGIN
--   IF NEW.title  IS DISTINCT FROM OLD.title
--      OR NEW.author IS DISTINCT FROM OLD.author
--      OR NEW.price  IS DISTINCT FROM OLD.price
--   THEN
--     RAISE EXCEPTION 'Core book fields are immutable';
--   END IF;
--   RETURN NEW;
-- END; $$ LANGUAGE plpgsql;

-- DROP TRIGGER IF EXISTS trg_books_immutable ON books;
-- CREATE TRIGGER trg_books_immutable
-- BEFORE UPDATE ON books
-- FOR EACH ROW
-- WHEN (OLD.* IS DISTINCT FROM NEW.*)
-- EXECUTE FUNCTION books_immutable_fields();

-- Indexes
CREATE INDEX IF NOT EXISTS idx_books_title       ON books(title);
CREATE INDEX IF NOT EXISTS idx_books_author      ON books(author);
CREATE INDEX IF NOT EXISTS idx_books_active_true ON books(active) WHERE active;
CREATE INDEX IF NOT EXISTS idx_books_created_at  ON books(created_at DESC);

-----------------------------------------------------------------------
-- Schema for orders_db
-----------------------------------------------------------------------
\connect orders_db

-- Main orders table: contains order-level information
CREATE TABLE orders (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id  TEXT,                                        -- future customer reference
    status       TEXT NOT NULL DEFAULT 'pending'              -- pending, confirmed, shipped, cancelled
                 CHECK (status IN ('pending', 'confirmed', 'shipped', 'cancelled')),
    total_amount NUMERIC(12,2) NOT NULL DEFAULT 0            -- calculated total from order_items
                 CHECK (total_amount >= 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Order items table: line items for each book in an order
CREATE TABLE order_items (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id    BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    book_id     BIGINT NOT NULL,                             -- reference to Books service (no FK across DBs)
    book_title  TEXT   NOT NULL CHECK (length(btrim(book_title))  > 0),
    book_author TEXT   NOT NULL CHECK (length(btrim(book_author)) > 0),
    quantity    INT    NOT NULL CHECK (quantity > 0 AND quantity <= 100000),
    unit_price  NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),
    line_total  NUMERIC(12,2) GENERATED ALWAYS AS ((quantity::numeric * unit_price)::numeric(12,2)) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- updated_at trigger for orders
CREATE OR REPLACE FUNCTION set_orders_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_orders_set_updated_at ON orders;
CREATE TRIGGER trg_orders_set_updated_at
BEFORE UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION set_orders_updated_at();

-- Trigger to automatically update order total when order_items change
CREATE OR REPLACE FUNCTION update_order_total()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE orders 
    SET total_amount = (
        SELECT COALESCE(SUM(line_total), 0) 
        FROM order_items 
        WHERE order_id = COALESCE(NEW.order_id, OLD.order_id)
    )
    WHERE id = COALESCE(NEW.order_id, OLD.order_id);
    
    RETURN COALESCE(NEW, OLD);
END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_order_total_insert ON order_items;
CREATE TRIGGER trg_update_order_total_insert
AFTER INSERT ON order_items
FOR EACH ROW
EXECUTE FUNCTION update_order_total();

DROP TRIGGER IF EXISTS trg_update_order_total_update ON order_items;
CREATE TRIGGER trg_update_order_total_update
AFTER UPDATE ON order_items
FOR EACH ROW
EXECUTE FUNCTION update_order_total();

DROP TRIGGER IF EXISTS trg_update_order_total_delete ON order_items;
CREATE TRIGGER trg_update_order_total_delete
AFTER DELETE ON order_items
FOR EACH ROW
EXECUTE FUNCTION update_order_total();

-- Indexes for optimal performance
CREATE INDEX IF NOT EXISTS idx_orders_status      ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at  ON orders(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id  ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_book_id   ON order_items(book_id);
CREATE INDEX IF NOT EXISTS idx_order_items_created_at ON order_items(created_at DESC);
