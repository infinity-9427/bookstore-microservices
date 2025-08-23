import os
from dotenv import load_dotenv

load_dotenv()


class Config:
    # Database
    BOOKS_DB_DSN = os.getenv("BOOKS_DB_DSN")
    PORT = int(os.getenv("PORT", 8001))
    
    CLOUDINARY_CLOUD_NAME = os.getenv("CLOUDINARY_CLOUD_NAME")
    CLOUDINARY_API_KEY = os.getenv("CLOUDINARY_API_KEY")
    CLOUDINARY_API_SECRET = os.getenv("CLOUDINARY_API_SECRET")
    UPLOAD_PRESET = os.getenv("UPLOAD_PRESET")
    
    # Image settings
    MAX_IMAGE_SIZE = int(os.getenv("MAX_IMAGE_SIZE", 10485760))  # 10MB default
    ALLOWED_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp", "image/avif"]
    
    @classmethod
    def validate_required(cls):
        """Validate that all required environment variables are set"""
        if not cls.BOOKS_DB_DSN:
            raise ValueError("BOOKS_DB_DSN environment variable is required")
        
        # Cloudinary config is optional - only validate if one is set
        cloudinary_vars = [cls.CLOUDINARY_CLOUD_NAME, cls.CLOUDINARY_API_KEY, cls.CLOUDINARY_API_SECRET]
        if any(cloudinary_vars) and not all(cloudinary_vars):
            raise ValueError("All Cloudinary environment variables must be set: CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET")
    
    @classmethod 
    def has_cloudinary(cls) -> bool:
        """Check if Cloudinary is configured"""
        return all([cls.CLOUDINARY_CLOUD_NAME, cls.CLOUDINARY_API_KEY, cls.CLOUDINARY_API_SECRET])