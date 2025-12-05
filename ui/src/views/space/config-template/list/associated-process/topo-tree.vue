<template>
  <ul class="bind-topo-tree">
    <template v-for="(topoNode, index) in treeNodeList" :key="index">
      <li v-show="topoNode.topoVisible" class="tree-node-container">
        <div class="tree-node-item" @click="handleClickNode(topoNode)">
          <!-- 进程模板 templateProcess、进程实例 instanceProcess -->
          <template v-if="topoNode.topoProcess">
            <bk-checkbox
              v-model="topoNode.topoChecked"
              class="king-checkbox"
              :style="{ 'padding-left': 26 * topoNode.topoLevel + 22 + 'px' }"
              @change="handleCheckNode(topoNode)">
              <div class="text-content">{{ topoNode.topoName }}</div>
            </bk-checkbox>
          </template>

          <!-- 业务节点 -->
          <template v-else>
            <angle-down-fill
              :class="['angle-icon', topoNode.topoExpand && 'expanded']"
              :style="{ 'margin-left': 26 * topoNode.topoLevel + 'px' }" />
            <!-- 集群 -->
            <span v-if="topoNode.topoType === 'set'" class="word-icon">{{ $t('集') }}</span>
            <!-- 服务模板 -->
            <span v-else-if="topoNode.topoType === 'serviceTemplate'" class="word-icon blue">{{ $t('模') }}</span>
            <!-- 模块 -->
            <span v-else-if="topoNode.topoType === 'module'" class="word-icon">{{ $t('模') }}</span>
            <!-- 服务实例 -->
            <span v-else-if="topoNode.topoType === 'serviceInstance'" class="word-icon">{{ $t('实') }}</span>

            <div class="text-content">
              <bk-overflow-title type="tips">
                <span>{{ topoNode.topoName }}</span>
              </bk-overflow-title>
            </div>

            <span v-if="topoNode.topoProcessCount !== undefined" class="process-count">
              {{ topoNode.topoProcessCount }}
            </span>
          </template>
        </div>

        <div
          v-if="topoNode.topoLoading"
          class="tree-node-loading"
          :style="{ 'padding-left': 26 * (topoNode.topoLevel + 1) + 22 + 'px' }">
          <Spinner class="spinner-icon" />
          <span class="loading-text">{{ $t('加载中') }}</span>
        </div>

        <TopoTree
          v-if="topoNode.child && topoNode.child.length"
          v-show="topoNode.topoExpand"
          v-model:template-process="templateProcess"
          v-model:instance-process="instanceProcess"
          :node-list="topoNode.child"
          :bk-biz-id="bkBizId"
          @checked="handleCheckNode" />
      </li>
    </template>
  </ul>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import {
    getServiceInstanceFormModule,
    getProcessListFormServiceInstance,
    getProcessListFormServiceTemplate,
  } from '../../../../../api/config-template';
  import type { ITopoTreeNode, IProcessPreviewItem } from '../../../../../../types/config-template';
  import { AngleDownFill, Spinner } from 'bkui-vue/lib/icon';

  defineOptions({
    name: 'TopoTree',
  });
  const props = defineProps<{
    nodeList: ITopoTreeNode[];
    bkBizId: string;
    templateProcess: IProcessPreviewItem[];
    instanceProcess: IProcessPreviewItem[];
  }>();
  const emits = defineEmits(['checked', 'update:templateProcess', 'update:instanceProcess']);
  const treeNodeList = ref<ITopoTreeNode[]>(props.nodeList);
  const templateProcess = ref<IProcessPreviewItem[]>(props.templateProcess);
  const instanceProcess = ref<IProcessPreviewItem[]>(props.instanceProcess);

  watch(
    () => props.nodeList,
    (newVal: ITopoTreeNode[]) => {
      treeNodeList.value = newVal;
    },
    { deep: true },
  );

  const handleClickNode = async (topoNode: ITopoTreeNode) => {
    if (topoNode.topoProcess) return;

    topoNode.topoExpand = !topoNode.topoExpand;

    // 展开且没有子节点 → 请求接口
    if (topoNode.topoExpand && !topoNode.child?.length) {
      try {
        topoNode.topoLoading = true;
        // ------- 1. 模块 → 服务实例 -------
        if (topoNode.topoType === 'module') {
          // 需要展开的模块没有服务实例子节点，根据模块查询服务实例列表
          const res = await getServiceInstanceFormModule(props.bkBizId, topoNode.bk_inst_id!);
          topoNode.child = res.service_instances.map((item: any) => {
            return {
              child: [],
              topoParentName: topoNode.topoName,
              topoVisible: true,
              topoExpand: false,
              topoLoading: false,
              topoLevel: topoNode.topoLevel + 1,
              topoName: item.name,
              topoProcessCount: item.process_count,
              topoProcess: false,
              topoType: 'serviceInstance',
              service_instance_id: item.id,
            };
          });
        }

        // ------- 2. 服务实例 → 实例进程 -------
        else if (topoNode.topoType === 'serviceInstance') {
          const res = await getProcessListFormServiceInstance(props.bkBizId, topoNode.service_instance_id!);

          topoNode.child = res.process_instances.map((item: any) => {
            return {
              topoParentName: topoNode.topoName,
              topoVisible: true,
              topoExpand: false,
              topoLoading: false,
              topoLevel: topoNode.topoLevel + 1,
              topoName: item.property.bk_process_name,
              topoProcess: true,
              topoType: 'instanceProcess',
              topoChecked: false,
              processId: item.property.bk_process_id,
            };
          });
          instanceProcess.value.forEach((process) => {
            const findNode = topoNode.child.find((node) => process.__IS_RECOVER && node.processId === process.id);
            if (findNode) {
              findNode.topoChecked = true;
              process.topoNode = findNode;
              emits('update:instanceProcess', instanceProcess.value);
            }
          });
        }
        // ------- 3. 服务模板 → 模板进程 -------
        else if (topoNode.topoType === 'serviceTemplate') {
          const res = await getProcessListFormServiceTemplate(props.bkBizId, topoNode.service_template_id);
          topoNode.child = res.process_templates.map((item: any) => {
            return {
              topoParentName: topoNode.topoName,
              topoVisible: true,
              topoExpand: false,
              topoLoading: false,
              topoLevel: topoNode.topoLevel + 1,
              topoName: item.bk_process_name,
              topoProcessCount: item.process_count,
              topoProcess: true,
              topoType: 'templateProcess',
              topoChecked: false,
              processId: item.id,
            };
          });
          templateProcess.value.forEach((process) => {
            const findNode = topoNode.child.find((node) => process.__IS_RECOVER && node.processId === process.id);
            if (findNode) {
              process.topoNode = findNode;
              findNode.topoChecked = true;
              emits('update:templateProcess', templateProcess.value);
            }
          });
        }
      } catch (e) {
        console.error(e);
      } finally {
        topoNode.topoLoading = false;
      }
    }

    // 有子节点 → 展示
    if (topoNode.child?.length) {
      topoNode.child.forEach((node) => {
        node.topoVisible = true;
      });
    }
  };

  const handleCheckNode = (topoNode: ITopoTreeNode) => {
    emits('checked', topoNode);
  };
</script>

<style scoped lang="scss">
  .bind-topo-tree {
    .tree-node-container {
      .tree-node-item {
        display: flex;
        align-items: center;
        height: 36px;
        line-height: 20px;
        font-size: 14px;
        color: #63656e;
        cursor: pointer;
        transition: background-color 0.2s;
        &:hover {
          background-color: #f0f1f5;
          transition: background-color 0.2s;
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

        .word-icon {
          flex-shrink: 0;
          width: 20px;
          text-align: center;
          font-size: 12px;
          color: #fff;
          background-color: #c4c6cc;
          border-radius: 50%;
          margin-right: 7px;

          &.blue {
            background-color: #97aed6;
          }
        }

        .text-content {
          width: 100%;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }

        .process-count {
          flex-shrink: 0;
          padding: 0 2px;
          min-width: 30px;
          line-height: 16px;
          text-align: center;
          font-size: 12px;
          color: #979ba5;
          background: #f0f1f5;
          border-radius: 2px;
        }

        .king-checkbox {
          display: flex;
          align-items: center;
          width: 100%;
          height: 100%;

          :deep(.bk-checkbox) {
            flex-shrink: 0;
          }
          :deep(.bk-checkbox-text) {
            margin-left: 8px;
            width: calc(100% - 24px);
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
    }
  }
</style>
