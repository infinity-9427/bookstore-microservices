-- Create service databases (owned by default 'postgres' superuser)
-- These run once on first init of the Docker volume.

CREATE DATABASE books_db;
CREATE DATABASE orders_db;

-- Create service users with limited privileges (separate per service)
CREATE USER books_user  WITH PASSWORD 'books_password';
CREATE USER orders_user WITH PASSWORD 'orders_password';

-- Enforce "no shared DB access": only each app user can CONNECT to its DB
REVOKE CONNECT ON DATABASE books_db  FROM PUBLIC;
REVOKE CONNECT ON DATABASE orders_db FROM PUBLIC;
GRANT  CONNECT ON DATABASE books_db  TO books_user;
GRANT  CONNECT ON DATABASE orders_db TO orders_user;

-- Set deterministic timezone per DB
ALTER DATABASE books_db  SET timezone TO 'UTC';
ALTER DATABASE orders_db SET timezone TO 'UTC';

-----------------------------------------------------------------------
-- BOOKS DB privileges (least privilege: no CREATE at runtime)
-----------------------------------------------------------------------
\connect books_db

-- Lock down the public schema, then grant only what the app needs
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO books_user;

-- DML + sequence usage on existing objects
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA public TO books_user;
GRANT USAGE,  SELECT                 ON ALL SEQUENCES IN SCHEMA public TO books_user;

-- Ensure future objects created by 'postgres' are accessible to the app
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO books_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE,  SELECT                 ON SEQUENCES TO books_user;

-----------------------------------------------------------------------
-- ORDERS DB privileges (least privilege: no CREATE at runtime)
-----------------------------------------------------------------------
\connect orders_db

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO orders_user;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA public TO orders_user;
GRANT USAGE,  SELECT                 ON ALL SEQUENCES IN SCHEMA public TO orders_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO orders_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE,  SELECT                 ON SEQUENCES TO orders_user;

-- NOTE:
-- If you intentionally run DB migrations from the apps, also grant:
--   GRANT CREATE ON SCHEMA public TO <service_user>;
-- Otherwise keep as-is for stricter security.
