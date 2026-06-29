FROM ghcr.io/astral-sh/uv:python3.12-alpine

RUN apk --update --no-cache add ca-certificates bash \
    libxml2 libxslt \
    build-base python3-dev libxml2-dev libxslt-dev

COPY build/bk-bscp/bk-bscp-gsekit-migration/bk-bscp-gsekit-migration /app/bk-bscp-gsekit-migration
COPY render/python /app/render/python

WORKDIR /app/render/python
RUN uv sync --frozen
RUN .venv/bin/python3 -c "import mako, lxml"

WORKDIR /app
ENV BSCP_PYTHON_RENDER_PATH=/app/render/python

ENTRYPOINT ["/app/bk-bscp-gsekit-migration"]
