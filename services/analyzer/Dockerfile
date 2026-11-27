# syntax=docker/dockerfile:1

FROM python:3.15.2-alpine3.23 AS build

ENV PYTHONDONTWRITEBYTECODE=1

WORKDIR /app

# The base image predates Alpine's zlib and openssl fixes.
RUN apk upgrade --no-cache zlib libcrypto3 libssl3 \
    && adduser -D -H -g "" appuser

COPY requirements.txt .
# pip, busybox, apk-tools, musl-utils and scanelf are build-only (pip's vendored urllib3, msgpack and setuptools carry HIGH CVEs).
# alpine-baselayout and ca-certificates need busybox's sh, so only their data and CA bundle packages stay.
RUN pip install --no-cache-dir --require-hashes --only-binary :all: -r requirements.txt \
    && pip uninstall --yes pip \
    && apk add --no-cache alpine-baselayout-data ca-certificates-bundle \
    && find /etc/ssl/certs -type l -delete \
    && sed 's|^|/|' /etc/busybox-paths.d/busybox | /bin/busybox xargs /bin/busybox rm -f \
    && /sbin/apk del --no-cache --scripts=no \
        alpine-baselayout apk-tools busybox busybox-binsh ca-certificates libapk musl-utils scanelf ssl_client

FROM scratch
LABEL org.opencontainers.image.licenses="Elastic-2.0"
COPY --from=build / /

ENV PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ENV LANG=C.UTF-8
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

WORKDIR /app
COPY main.py LICENSE.md THIRD-PARTY-NOTICES.md ./
COPY analysis/ analysis/
COPY api/ api/
COPY core/ core/
COPY prompts/ prompts/
COPY providers/ providers/
COPY tools/ tools/

USER appuser

CMD ["python", "main.py"]
