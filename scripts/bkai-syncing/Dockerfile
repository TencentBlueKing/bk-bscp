# 以仓库根目录为构建上下文：make bkai-image
# SKILL.md 从 .agents/skills/ 复制，本地 IDE 与 AIDev 使用同一份内容
ARG BKAI_INIT_IMAGE=hub.bktencent.com/blueking/bk-aidev-init:0.1.6
FROM ${BKAI_INIT_IMAGE}

WORKDIR /app
COPY --chown=10001:10001 scripts/bkai-syncing/package /app/package
COPY --chown=10001:10001 .agents/skills/bkbscp-kv-config/SKILL.md /app/package/skills/bkbscp-kv-config/SKILL.md
COPY --chown=10001:10001 .agents/skills/bkbscp-file-config/SKILL.md /app/package/skills/bkbscp-file-config/SKILL.md

CMD ["sync", "--confirm", "-f", "/app/package/bkai.yaml"]
