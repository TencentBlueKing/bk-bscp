# 使用 uv 镜像，构建阶段通过 uv sync 预装并锁定 Python 依赖
FROM ghcr.io/astral-sh/uv:python3.12-alpine

# 只安装运行时库。编译器不能单独成层：扫描会检查每一层，
# build-base 带进来的 C++ 头文件版本号会被误判成内网 IP。
RUN apk --update --no-cache add ca-certificates bash vim curl \
    libxml2 libxslt

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
# AIDev Skill 同步包，由 Helm Job 的 initContainer 复制给 bk-aidev-init 使用
COPY scripts/bkai-syncing/package /bk-bscp/bkai
COPY .agents/skills/bkbscp-kv-config/SKILL.md /bk-bscp/bkai/skills/bkbscp-kv-config/SKILL.md
COPY .agents/skills/bkbscp-file-config/SKILL.md /bk-bscp/bkai/skills/bkbscp-file-config/SKILL.md
COPY .agents/skills/bkbscp-process-config/SKILL.md /bk-bscp/bkai/skills/bkbscp-process-config/SKILL.md
# 复制 Python 模块到镜像中。tests 与本地 .venv 由 .dockerignore 排除。
COPY render/python /bk-bscp/render/python
WORKDIR /bk-bscp/render/python
# 编译 lxml 需要编译器。安装、同步、删除必须在同一次 RUN 里完成，
# 否则编译头文件和 uv 缓存会留在中间层，被敏感信息扫描扫到。
ENV UV_NO_CACHE=1
RUN apk add --no-cache --virtual .build-deps build-base python3-dev libxml2-dev libxslt-dev \
    && uv sync --frozen \
    && .venv/bin/python3 -c "import mako, lxml" \
    && apk del .build-deps \
    && rm -rf /root/.cache
WORKDIR /bk-bscp
ENV BSCP_PYTHON_RENDER_PATH=/bk-bscp/render/python
ENTRYPOINT ["/bk-bscp/bk-bscp-ui"]
CMD []