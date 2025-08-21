import os
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, Session
from sqlalchemy.exc import SQLAlchemyError
from database import Base


class DatabaseManager:
    def __init__(self):
        self.engine = None
        self.SessionLocal = None
        self._initialize_database()

    def _initialize_database(self):
        database_url = os.getenv("BOOKS_DB_DSN")
        if not database_url:
            raise ValueError("BOOKS_DB_DSN environment variable is required")
        
        self.engine = create_engine(
            database_url,
            pool_pre_ping=True,
            pool_recycle=300
        )
        self.SessionLocal = sessionmaker(
            autocommit=False, 
            autoflush=False, 
            bind=self.engine
        )

    def get_session(self) -> Session:
        return self.SessionLocal()

    def check_health(self) -> bool:
        try:
            with self.get_session() as session:
                session.execute("SELECT 1")
                return True
        except SQLAlchemyError:
            return False


db_manager = DatabaseManager()


def get_db_session():
    with db_manager.get_session() as session:
        try:
            yield session
        finally:
            session.close()