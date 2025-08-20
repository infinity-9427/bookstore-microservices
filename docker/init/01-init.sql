CREATE USER books_user  WITH PASSWORD 'books_pass';
CREATE DATABASE books_db  OWNER books_user;
GRANT ALL PRIVILEGES ON DATABASE books_db  TO books_user;

CREATE USER orders_user WITH PASSWORD 'orders_pass';
CREATE DATABASE orders_db OWNER orders_user;
GRANT ALL PRIVILEGES ON DATABASE orders_db TO orders_user;
