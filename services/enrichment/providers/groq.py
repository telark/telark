"""Groq enrichment provider (OpenAI-compatible, instructor-backed)."""

from config import GROQ_MODEL

from .base import InstructorProvider
from .config import GROQ_BASE_URL, GROQ_RETRIES


class GroqProvider(InstructorProvider):
    name = "groq"
    base_url = GROQ_BASE_URL
    model = GROQ_MODEL
    retries = GROQ_RETRIES
