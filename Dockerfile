# 使用 uv 镜像，构建阶段通过 uv sync 预装并锁定 Python 依赖
FROM ghcr.io/astral-sh/uv:python3.12-alpine

RUN apk --update --no-cache add ca-certificates bash vim curl \
    # Install runtime libraries for lxml (needed at runtime)
    libxml2 libxslt \
    # Install build dependencies for Python packages (needed for compiling lxml)
    build-base python3-dev libxml2-dev libxslt-dev

COPY build/bk-bscp/bk-bscp-ui/bk-bscp-ui /bk-bscp/
COPY build/bk-bscp/bk-bscp-apiserver/bk-bscp-apiserver /bk-bscp/
COPY build/bk-bscp/bk-bscp-authserver/bk-bscp-authserver /bk-bscp/
COPY build/bk-bscp/bk-bscp-cacheservice/bk-bscp-cacheservice /bk-bscp/
COPY build/bk-bscp/bk-bscp-configserver/bk-bscp-configserver /bk-bscp/
COPY build/bk-bscp/bk-bscp-dataservice/bk-bscp-dataservice /bk-bscp/
COPY build/bk-bscp/bk-bscp-feedserver/bk-bscp-feedserver /bk-bscp/
COPY build/bk-bscp/bk-bscp-feedproxy/bk-bscp-feedproxy /bk-bscp/
COPY build/bk-bscp/bk-bscp-vaultserver/bk-bscp-vaultserver /bk-bscp/
COPY build/bk-bscp/bk-bscp-vaultserver/vault /bk-bscp/
COPY build/bk-bscp/bk-bscp-vaultserver/vault-sidecar /bk-bscp/
COPY build/bk-bscp/bk-bscp-vaultserver/vault-plugins/bk-bscp-secret /etc/vault/vault-plugins/
# 把 system_bk_bscp.json 放到容器内 /bk-bscp/etc/itsm/
COPY scripts/itsm-templates/system_bk_bscp.json /bk-bscp/etc/itsm/system_bk_bscp.json
# 复制 Python 模块到镜像中
COPY render/python /bk-bscp/render/python
WORKDIR /bk-bscp/render/python
RUN uv sync --frozen
RUN .venv/bin/python3 -c "import mako, lxml"
WORKDIR /bk-bscp
ENV BSCP_PYTHON_RENDER_PATH=/bk-bscp/render/python
ENTRYPOINT ["/bk-bscp/bk-bscp-ui"]
CMD []