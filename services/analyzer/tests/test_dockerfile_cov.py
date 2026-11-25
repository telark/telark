"""Checks the Dockerfile. It names its sources instead of `COPY . .` (Sonar docker:S6470), so a new
module or package must be added there too.

Run: python -m pytest tests/test_dockerfile_cov.py
"""

from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCKERFILE = (ROOT / "Dockerfile").read_text()


def copied_sources():
    sources = set()
    for line in DOCKERFILE.splitlines():
        if line.startswith("COPY ") and "--from" not in line:
            sources.update(line.split()[1:-1])
    return sources


def test_image_copies_every_module():
    modules = {p.name for p in ROOT.glob("*.py")}
    packages = {f"{p.name}/" for p in ROOT.iterdir() if p.is_dir() and p.name != "tests" and any(p.glob("*.py"))}
    missing = (modules | packages) - copied_sources()
    assert not missing, f"add to the Dockerfile COPY list: {sorted(missing)}"


def test_runtime_user_owns_no_app_file():
    assert "chown" not in DOCKERFILE, "copy files root-owned and read-only (Sonar docker:S6504)"


def test_final_stage_starts_from_scratch():
    stages = [line for line in DOCKERFILE.splitlines() if line.startswith("FROM ")]
    assert stages[-1] == "FROM scratch", "the final stage copies the build root without busybox and apk-tools"


def test_build_stage_removes_shell_and_apk():
    build_stage = DOCKERFILE.split("FROM scratch")[0]
    removed = set(build_stage.partition("apk del")[2].split())
    assert {"busybox", "apk-tools", "musl-utils", "scanelf"} <= removed, "the image ships no shell, package manager or ELF tools"
