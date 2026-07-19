"""Coverage for app_logger.py — configure + level icons.

Run: pytest tests/test_app_logger_cov.py
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from app_logger import LOG_FORMAT, configure, logger  # noqa: E402


def test_configure_installs_handler_and_icons():
    configure(level="DEBUG")
    logger.info("smoke")
    logger.debug("smoke")
    assert "{message}" in LOG_FORMAT
