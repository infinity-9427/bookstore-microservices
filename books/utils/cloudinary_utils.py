# utils/cloudinary_utils.py
from typing import Optional
from models import ImageData
from config import Config

try:
    import cloudinary
    import cloudinary.uploader
    CLOUDINARY_AVAILABLE = True
except Exception:
    CLOUDINARY_AVAILABLE = False


def init_cloudinary_if_configured() -> bool:
    """Initialize Cloudinary only when all creds exist. Returns True if configured."""
    if not CLOUDINARY_AVAILABLE:
        return False
    if not Config.has_cloudinary():
        return False

    cloudinary.config(
        cloud_name=Config.CLOUDINARY_CLOUD_NAME,
        api_key=Config.CLOUDINARY_API_KEY,
        api_secret=Config.CLOUDINARY_API_SECRET,
        secure=True,
    )
    return True


def upload_book_cover(content: bytes, ) -> ImageData:

    if not (CLOUDINARY_AVAILABLE and Config.has_cloudinary()):
        raise RuntimeError("Cloudinary is not available or not configured")

    result = cloudinary.uploader.upload(
        content,
        folder=Config.UPLOAD_PRESET,
        format="webp",
        quality="auto:best",
        fetch_format="auto",
        transformation=[
            {"width": 800, "height": 1200, "crop": "limit"},
            {"quality": "auto:best"},
            {"format": "webp"},
        ],
        resource_type="image",
        public_id_prefix="book_cover_",
        unique_filename=True,
        overwrite=False,
        tags=["book_cover"],
    )

    return ImageData(url=result["secure_url"], public_id=result["public_id"])


def delete_cloudinary_image(public_id: str) -> bool:
    """Best-effort delete; returns True on success."""
    if not (CLOUDINARY_AVAILABLE and Config.has_cloudinary()):
        return False
    try:
        import cloudinary.uploader  # local import to ensure module exists at runtime
        result = cloudinary.uploader.destroy(public_id)
        return result.get("result") == "ok"
    except Exception:
        return False
