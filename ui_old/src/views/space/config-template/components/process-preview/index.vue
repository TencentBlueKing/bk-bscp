<template>
  <div class="preview-wrap">
    <div class="head">
      <div class="head-left">
        <div class="close-btn" @click="emits('close')">
          <angle-down-line class="close-icon" />
        </div>
        <span class="title">{{ $t('预览') }}</span>
      </div>
      <TreeSelect ref="treeSelectRef" :tree-data="topoData" @selected="handleSelectProcess" />
    </div>
    <bk-loading class="preview-content" :loading="contentLoading" color="#242424">
      <CodeEditor
        v-if="instId"
        :model-value="previewContent"
        :editable="false"
        line-numbers="off"
        :minimap="false"
        :vertical-scrollbar-size="0"
        :horizon-scrollbar-size="0"
        render-line-highlight="none"
        :render-indent-guides="false"
        :folding="false"
        language="python" />
      <bk-exception
        v-else
        class="exception-wrap-item exception-gray"
        :description="$t('请先选择进程实例')"
        scene="part"
        type="empty" />
    </bk-loading>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onBeforeUnmount, onMounted } from 'vue';
  import { AngleDownLine } from 'bkui-vue/lib/icon';
  import { getProcessInstanceTopoTreeNodes, previewConfig } from '../../../../../api/config-template';
  import CodeEditor from '../../../../../components/code-editor/index.vue';
  import TreeSelect from './tree-select.vue';
  import { ITopoTreeNode, ITopoTreeNodeRes } from '../../../../../../types/config-template';

  const emits = defineEmits(['close']);
  const props = defineProps<{
    bkBizId: string;
    configContent: string;
  }>();

  const codeEditorRef = ref();
  const previewContent = ref('');
  const topoData = ref<ITopoTreeNode[]>([]);
  const contentLoading = ref(false);
  const instId = ref(0);

  onMounted(() => {
    loadTopoTreeData();
  });

  onBeforeUnmount(() => {
    if (codeEditorRef.value) {
      codeEditorRef.value.destroy();
    }
  });

  const loadTopoTreeData = async () => {
    try {
      const res = await getProcessInstanceTopoTreeNodes(props.bkBizId);
      topoData.value = filterTopoData(res.biz_topo_nodes);
    } catch (e) {
      console.warn(e);
    }
  };

  // 处理topo树结构
  const filterTopoData = (nodes: ITopoTreeNodeRes[], topoLevel = -1, parent: ITopoTreeNode | null = null) => {
    topoLevel += 1;
    return nodes.map((node) => {
      const topo: ITopoTreeNode = {
        child: [], // 递归填充
        topoParentName: parent?.topoName || '',
        topoParent: parent,
        topoVisible: true,
        topoExpand: false,
        topoLoading: false,
        topoLevel,
        topoProcess: node.bk_obj_id === 'process',
        topoType: node.bk_obj_id,
        topoProcessCount: node.process_count,
        topoChecked: false,
        topoName: node.bk_inst_name,
        service_template_id: node.service_template_id,
        bk_inst_id: node.bk_inst_id,
      };
      if (node.child?.length) {
        topo.child = filterTopoData(node.child, topoLevel, topo);
      }
      return topo;
    });
  };

  const handleSelectProcess = async (id: number) => {
    if (!id) return;
    try {
      contentLoading.value = true;
      instId.value = id;
      const data = {
        templateContent: props.configContent,
        ccProcessId: id,
      };
      const res = await previewConfig(props.bkBizId, data);
      previewContent.value = res.content;
    } catch (error) {
      console.error(error);
    } finally {
      contentLoading.value = false;
    }
  };

  defineExpose({
    reloadPreview: () => {
      handleSelectProcess(instId.value);
    },
  });
</script>

<style scoped lang="scss">
  .preview-wrap {
    flex-shrink: 0;
    width: 417px;
    height: 100%;
    border-radius: 4px;
    background: #f5f7fa;
    .head {
      display: flex;
      justify-content: space-between;
      align-items: center;
      height: 40px;
      line-height: 40px;
      background: #2e2e2e;
      .head-left {
        display: flex;
        align-items: center;
      }
      .close-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 30px;
        height: 40px;
        background: #478efd;
        cursor: pointer;
        .close-icon {
          color: #ffffff;
          font-size: 14px;
          transform: rotate(-90deg);
        }
      }
      .title {
        margin-left: 8px;
        font-size: 14px;
        color: #e6e6e6;
      }
      .process-select {
        width: 260px;
        margin-right: 16px;
        :deep(.bk-input) {
          height: 26px;
          line-height: 26px;
          border: 1px solid #575757;
          input {
            background: #2e2e2e;
            color: #b3b3b3;
          }
        }
        .select-prefix {
          padding: 0 8px;
          background: #3d3d3d;
          color: #b3b3b3;
        }
      }
    }
    .preview-content {
      height: calc(100% - 40px);
      background: #242424;
      .exception-wrap-item {
        padding-top: 100px;
        :deep(.bk-exception-img) {
          height: 150px;
        }
        :deep(.bk-exception-description) {
          font-size: 14px;
        }
      }
    }
  }
</style>
