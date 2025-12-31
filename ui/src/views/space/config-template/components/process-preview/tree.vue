<template>
  <ul class="select-tree-container">
    <li v-for="(topoNode, index) in nodeList" :key="index" v-show="topoNode.topoVisible" class="tree-node-container">
      <div
        class="tree-node-item"
        :class="{ 'checked-node-item': topoNode.topoChecked }"
        :style="{ paddingLeft: `${8 + 10 * topoNode.topoLevel}px` }"
        @click="handleClickNode(topoNode)">
        <!-- 非叶子节点 -->
        <div v-if="!topoNode.topoProcess" class="icon-container">
          <angle-down-fill :class="['angle-icon', topoNode.topoExpand && 'expanded']" />
        </div>
        <div class="node-type-icon" :class="{ 'process-type': topoNode.topoProcess }">
          {{ nodeTypeMap[topoNode.topoLevel] }}
        </div>

        <div class="text-content">
          <bk-overflow-title>{{ topoNode.topoName }}</bk-overflow-title>
        </div>

        <div v-if="topoNode.topoProcessCount !== undefined" class="node-tag process-count">
          {{ topoNode.topoProcessCount }}
        </div>
      </div>
      <TopoTree
        v-if="topoNode.child && topoNode.child.length"
        v-show="topoNode.topoExpand"
        :node-list="topoNode.child"
        @checked="handleCheckNode" />
    </li>
  </ul>
</template>

<script lang="ts" setup>
  import { useI18n } from 'vue-i18n';
  import { AngleDownFill } from 'bkui-vue/lib/icon';
  import type { ITopoTreeNode } from '../../../../../../types/config-template';

  defineOptions({
    name: 'TopoTree',
  });
  defineProps<{
    nodeList: ITopoTreeNode[];
  }>();
  const emit = defineEmits(['checked']);
  const { t } = useI18n();

  const nodeTypeMap = [t('集'), t('模'), t('实'), t('进')];

  const handleClickNode = async (topoNode: ITopoTreeNode) => {
    // 点击叶子（进程）
    if (topoNode.topoProcess) {
      emit('checked', topoNode);
      return;
    }
    topoNode.topoExpand = !topoNode.topoExpand;
    topoNode.child.forEach((node) => {
      node.topoVisible = true;
    });
  };

  const handleCheckNode = (topoNode: ITopoTreeNode) => {
    emit('checked', topoNode);
  };
</script>

<style scoped lang="scss">
  .select-tree-container {
    .tree-node-container {
      .tree-node-item {
        display: flex;
        align-items: center;
        height: 36px;
        line-height: 20px;
        padding-right: 8px;
        font-size: 14px;
        cursor: pointer;
        color: #e1ecff;
        transition:
          color 0.2s,
          background-color 0.2s;

        .icon-container {
          flex-shrink: 0;
          display: flex;
          justify-content: center;
          align-items: center;
          width: 20px;
          height: 20px;
          margin-right: 4px;

          .icon-right-shape {
            font-size: 14px;
            color: #a3c5fd;
            transition:
              color 0.2s,
              transform 0.2s;

            &.expanded {
              transform: rotate(90deg);
            }
          }
          .angle-icon {
            flex-shrink: 0;
            font-size: 14px;
            color: #63656e;
            cursor: pointer;
            margin-right: 6px;
            transition: transform 0.2s;
            transform: rotate(90deg);
            &.expanded {
              transform: rotate(180deg);
              transition: transform 0.2s;
            }
          }

          .svg-icon {
            width: 14px;
            height: 14px;
          }
        }

        .node-type-icon {
          flex-shrink: 0;
          width: 18px;
          height: 18px;
          line-height: 18px;
          margin-right: 6px;
          text-align: center;
          border-radius: 9px;
          font-size: 12px;
          color: #383838;
          background-color: #c4c6cc;

          &.process-type {
            margin-left: 24px;
          }
        }

        .text-content {
          width: 100%;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }

        .node-tag {
          flex-shrink: 0;
          min-width: 16px;
          padding: 0 2px;
          line-height: 16px;
          font-size: 12px;
          text-align: center;
        }

        .node-tag {
          flex-shrink: 0;
          min-width: 16px;
          padding: 0 2px;
          line-height: 16px;
          font-size: 12px;
          text-align: center;
          border-radius: 2px;
          color: #e1ecff;
          background: #474747;
        }

        .process-unmanaged {
          padding: 0 6px;
          color: #ad4d3e;
          background: #412525;
        }

        &:hover {
          color: #e1ecff;
          background-color: #415782;

          .icon-right-shape {
            color: #e1ecff;
          }

          .node-type-icon {
            background-color: #e1ecff;
          }
        }

        &.checked-node-item {
          color: #e1ecff;
          background-color: #346;

          .node-type-icon {
            background-color: #fff;
          }
        }
      }
    }
  }
  .tree-node-loading {
    display: flex;
    align-items: center;
    width: 100%;
    height: 36px;
    .spinner-icon {
      font-size: 16px;
    }
    .loading-text {
      font-size: 12px;
      padding-left: 4px;
      color: #a3c5fd;
    }
  }
</style>
