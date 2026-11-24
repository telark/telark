# syntax=docker/dockerfile:1

FROM python:3.15.2-alpine3.23
LABEL org.opencontainers.image.licenses="Elastic-2.0"

ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

WORKDIR /app

# The base image predates Alpine's zlib and openssl fixes.
RUN apk upgrade --no-cache zlib libcrypto3 libssl3

COPY requirements.txt .
# pip is install-only, and its vendored urllib3, msgpack and setuptools carry HIGH CVEs.
RUN pip install --no-cache-dir --require-hashes --only-binary :all: -r requirements.txt \
    && pip uninstall --yes pip

COPY main.py LICENSE.md THIRD-PARTY-NOTICES.md ./
COPY analysis/ analysis/
COPY api/ api/
COPY core/ core/
COPY prompts/ prompts/
COPY providers/ providers/
COPY tools/ tools/

RUN adduser -D -g "" appuser && chown -R appuser /app
USER appuser

CMD ["python", "main.py"]
