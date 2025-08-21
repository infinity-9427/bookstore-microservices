import os
from dotenv import load_dotenv

load_dotenv()


def get_config():
    books_db_dsn = os.getenv("BOOKS_DB_DSN")
    port = int(os.getenv("PORT", 8001))
    
    if not books_db_dsn:
        raise ValueError("BOOKS_DB_DSN environment variable is required")
    
    return {
        "database_url": books_db_dsn,
        "port": port
    }