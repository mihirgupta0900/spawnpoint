import logging
import sys

logger = logging.getLogger("spawnpoint")


def setup_logging(debug: bool = False):
    level = logging.DEBUG if debug else logging.WARNING
    handler = logging.StreamHandler(sys.stderr)
    handler.setFormatter(logging.Formatter("[%(levelname)s] %(message)s"))
    logger.setLevel(level)
    logger.addHandler(handler)
