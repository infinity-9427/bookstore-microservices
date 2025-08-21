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
CREATE OR REPLACE FUNCTION books_immutable_fields()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.title  IS DISTINCT FROM OLD.title
     OR NEW.author IS DISTINCT FROM OLD.author
     OR NEW.price  IS DISTINCT FROM OLD.price
  THEN
    RAISE EXCEPTION 'Core book fields are immutable';
  END IF;
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_books_immutable ON books;
CREATE TRIGGER trg_books_immutable
BEFORE UPDATE ON books
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION books_immutable_fields();

-- Indexes
CREATE INDEX IF NOT EXISTS idx_books_title       ON books(title);
CREATE INDEX IF NOT EXISTS idx_books_author      ON books(author);
CREATE INDEX IF NOT EXISTS idx_books_active_true ON books(active) WHERE active;
CREATE INDEX IF NOT EXISTS idx_books_created_at  ON books(created_at DESC);

-----------------------------------------------------------------------
-- Schema for orders_db
-----------------------------------------------------------------------
\connect orders_db

CREATE TABLE orders (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    book_id     BIGINT NOT NULL,                             -- reference to Books service (no FK across DBs)
    book_title  TEXT   NOT NULL CHECK (length(btrim(book_title))  > 0),
    book_author TEXT   NOT NULL CHECK (length(btrim(book_author)) > 0),
    quantity    INT    NOT NULL CHECK (quantity > 0 AND quantity <= 100000),
    unit_price  NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(12,2) GENERATED ALWAYS AS ((quantity::numeric * unit_price)::numeric(12,2)) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes tuned for reads and recent listings
CREATE INDEX IF NOT EXISTS idx_orders_book_id    ON orders(book_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);
