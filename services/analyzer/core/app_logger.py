"""Custom logger with fancy formatting (colors, level icons, timestamps)."""

import sys

from loguru import logger

# format: timestamp | level icon + name | module:function:line | message
LOG_FORMAT = (
    "<green>{time:YYYY-MM-DD HH:mm:ss.SSS}</green> | "
    "<level>{level.icon}</level> <level>{level: <8}</level> | "
    "<cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> | "
    "<level>{message}</level>"
)


def _configure_level_icons() -> None:
    logger.level("DEBUG", icon="\u2699\ufe0f")      # gear
    logger.level("INFO", icon="\u2139\ufe0f")       # info
    logger.level("WARNING", icon="\u26a0\ufe0f")    # warning
    logger.level("ERROR", icon="\u274c")            # cross
    logger.level("CRITICAL", icon="\U0001f4a5")     # collision
    logger.level("TRACE", icon="\U0001f50d")        # magnifying glass


def configure(level: str = "INFO") -> None:
    _configure_level_icons()
    logger.remove()
    logger.add(
        sys.stderr,
        format=LOG_FORMAT,
        level=level,
        colorize=True,
    )


__all__ = ["logger", "configure", "LOG_FORMAT"]
