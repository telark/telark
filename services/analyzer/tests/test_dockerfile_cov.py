"""Checks that the image gets every module. The Dockerfile names its sources instead of
`COPY . .` (Sonar docker:S6470), so a new module or package must be added there too.

Run: python -m pytest tests/test_dockerfile_cov.py
"""

from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def copied_sources():
    sources = set()
    for line in (ROOT / "Dockerfile").read_text().splitlines():
        if line.startswith("COPY ") and "--from" not in line:
            sources.update(line.split()[1:-1])
    return sources


def test_image_copies_every_module():
    modules = {p.name for p in ROOT.glob("*.py")}
    packages = {f"{p.name}/" for p in ROOT.iterdir() if p.is_dir() and p.name != "tests" and any(p.glob("*.py"))}
    missing = (modules | packages) - copied_sources()
    assert not missing, f"add to the Dockerfile COPY list: {sorted(missing)}"
