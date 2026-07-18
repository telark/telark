"""Gemini enrichment provider (OpenAI-compatible, instructor-backed)."""

from config import GEMINI_MODEL

from .base import InstructorProvider
from .config import GEMINI_BASE_URL, GEMINI_RETRIES


class GeminiProvider(InstructorProvider):
    name = "gemini"
    base_url = GEMINI_BASE_URL
    model = GEMINI_MODEL
    retries = GEMINI_RETRIES
