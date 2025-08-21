-- Create service databases owned by the default 'postgres' superuser
-- (the docker image runs these once on first init of the data volume)

CREATE DATABASE books_db;
CREATE DATABASE orders_db;

-- Optional: set a default timezone per DB (uncomment if you want)
-- ALTER DATABASE books_db  SET timezone TO 'UTC';
-- ALTER DATABASE orders_db SET timezone TO 'UTC';
