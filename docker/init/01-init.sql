-- Create service databases owned by the default 'postgres' superuser
-- (the docker image runs these once on first init of the data volume)

CREATE DATABASE books_db;
CREATE DATABASE orders_db;

-- Create service users with limited privileges
CREATE USER books_user WITH PASSWORD 'books_password';
CREATE USER orders_user WITH PASSWORD 'orders_password';

-- Grant database access to respective users
GRANT CONNECT ON DATABASE books_db TO books_user;
GRANT CONNECT ON DATABASE orders_db TO orders_user;

-- Grant schema usage and creation privileges
\c books_db
GRANT USAGE, CREATE ON SCHEMA public TO books_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO books_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO books_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO books_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO books_user;

\c orders_db
GRANT USAGE, CREATE ON SCHEMA public TO orders_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO orders_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO orders_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO orders_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO orders_user;

-- Set default timezone per DB
ALTER DATABASE books_db SET timezone TO 'UTC';
ALTER DATABASE orders_db SET timezone TO 'UTC';
