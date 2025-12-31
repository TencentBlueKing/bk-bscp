<template>
  <div class="content-wrap">
    <div class="associated-wrap">
      <div class="header">
        <span class="title">
          {{ $t('关联进程实例') }}
        </span>
        <span class="line"></span>
        <span>{{ templateName }}</span>
      </div>
      <div class="associated-content">
        <div class="label">{{ $t('选择关联进程') }}</div>
        <bk-radio-group v-model="processType" type="card">
          <bk-radio-button label="by_topo">{{ $t('按业务拓扑') }}</bk-radio-button>
          <bk-radio-button label="by_service">{{ $t('按服务模版') }}</bk-radio-button>
        </bk-radio-group>
        <SearchInput v-model="searchValue" class="search-input" @search="handleSearch" />
        <bk-loading class="tree-loading" :loading="treeLoading">
          <template v-if="isShowProcessTree">
            <TopoTree
              class="topo-tree"
              v-show="processType === 'by_topo'"
              v-model:template-process="templateProcess"
              v-model:instance-process="instanceProcess"
              :node-list="topoTreeData"
              :bk-biz-id="bkBizId"
              @checked="handleCheckNode" />
            <TopoTree
              class="template-tree"
              v-show="processType === 'by_service'"
              v-model:template-process="templateProcess"
              v-model:instance-process="instanceProcess"
              :node-list="templateTreeData"
              :bk-biz-id="bkBizId"
              @checked="handleCheckNode" />
          </template>
          <TableEmpty v-else :is-search-empty="isSearchEmpty" @clear="handleClearSearch" />
        </bk-loading>
      </div>
    </div>
    <div class="preview-wrap">
      <div class="title">
        {{ $t('结果预览') }}
      </div>
      <div v-if="templateProcess.length + instanceProcess.length > 0" class="scroll-container">
        <!-- 模板进程 -->
        <ResultPreview
          v-show="templateProcess.length"
          :process="templateProcess"
          :is-template="true"
          @remove="removeTemplateProcess" />
        <!-- 实例进程 -->
        <ResultPreview
          v-show="instanceProcess.length"
          :process="instanceProcess"
          :is-template="false"
          @remove="removeInstanceProcess" />
      </div>
      <bk-exception
        v-else
        class="exception-wrap-item exception-part"
        :description="$t('请选择关联进程')"
        scene="part"
        type="empty" />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted, computed } from 'vue';
  import {
    getTopoTreeNodes,
    getServiceTemplateTreeNodes,
    getBindProcessInstance,
  } from '../../../../../api/config-template';
  import type {
    ITopoTreeNode,
    ITopoTreeNodeRes,
    ITemplateTreeNodeRes,
    IProcessPreviewItem,
  } from '../../../../../../types/config-template';
  import SearchInput from '../../../../../components/search-input.vue';
  import TopoTree from './topo-tree.vue';
  import ResultPreview from './result-preview.vue';
  import TableEmpty from '../../../../../components/table/table-empty.vue';

  const props = defineProps<{
    bkBizId: string;
    templateName: string;
    templateId: number;
  }>();
  const emits = defineEmits(['change']);

  const processType = ref('by_topo');
  const searchValue = ref('');
  const topoTreeData = ref<ITopoTreeNode[]>([]);
  const templateTreeData = ref<ITopoTreeNode[]>([]);
  const treeLoading = ref(false);
  const templateProcess = ref<IProcessPreviewItem[]>([]);
  const instanceProcess = ref<IProcessPreviewItem[]>([]);
  const searchTimer = ref();
  const isSearchEmpty = ref(false);

  onMounted(() => {
    loadAllTreeNodes();
  });

  const isShowProcessTree = computed(() => {
    const list = processType.value === 'by_topo' ? topoTreeData.value : templateTreeData.value;

    // 无搜索：只要有数据就展示
    if (searchValue.value === '') {
      return list.length > 0;
    }

    // 有搜索：只要某个节点可见即可
    return list.some((item) => item.topoVisible);
  });

  const loadAllTreeNodes = async () => {
    try {
      treeLoading.value = true;
      const [topoRes, templateRes, bindRes] = await Promise.all([
        getTopoTreeNodes(props.bkBizId),
        getServiceTemplateTreeNodes(props.bkBizId),
        getBindProcessInstance(props.bkBizId, props.templateId),
      ]);
      topoTreeData.value = filterTopoData(topoRes.biz_topo_nodes);
      filterTemplateData(templateRes.service_templates);
      recoverBindRelationship(bindRes);
    } catch (error) {
      console.error(error);
    } finally {
      treeLoading.value = false;
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
        topoProcess: false,
        topoType: '',
        topoProcessCount: node.process_count,
        topoChecked: false,
        topoName: node.bk_inst_name,
        service_template_id: node.service_template_id,
        bk_inst_id: node.bk_inst_id,
      };
      if (node.bk_obj_id === 'set') {
        topo.topoType = 'set';
      } else if (node.bk_obj_id === 'module') {
        topo.topoType = node.service_template_id ? 'serviceTemplate' : 'module';
      }
      if (node.child?.length) {
        topo.child = filterTopoData(node.child, topoLevel, topo);
      }
      return topo;
    });
  };
  // 处理服务模板树结构
  const filterTemplateData = (templateData: ITemplateTreeNodeRes[]) => {
    templateTreeData.value = templateData.map((item: ITemplateTreeNodeRes) => {
      return {
        child: [],
        topoParentName: item.name,
        topoParent: null,
        topoVisible: true,
        topoExpand: false,
        topoLoading: false,
        topoLevel: 0,
        topoName: item.name,
        topoProcess: false,
        topoType: 'serviceTemplate',
        topoChecked: false,
        service_template_id: item.id,
        topoProcessCount: item.process_count,
      };
    });
  };

  // 绑定关系回填
  const recoverBindRelationship = (bindRes: any) => {
    try {
      bindRes.template_processes.forEach((item: any) => {
        templateProcess.value.push({
          __IS_RECOVER: true,
          id: item.id,
          topoName: item.process_name,
          topoParentName: item.name,
        });
      });
      bindRes.instance_processes.forEach((item: any) => {
        instanceProcess.value.push({
          __IS_RECOVER: true,
          id: item.id,
          topoName: item.process_name,
          topoParentName: item.name,
        });
      });
    } catch (e) {
      console.error(e);
    }
  };

  // 选择进程节点
  const handleCheckNode = (topoNode: ITopoTreeNode) => {
    if (topoNode.topoType === 'templateProcess') {
      if (topoNode.topoChecked) {
        // 选择进程
        // 查看已选择的模板进程是否包含当前进程（业务拓扑、服务模板两棵树有重复的进程）
        let findItem;
        let findIndex;
        for (let i = 0; i < templateProcess.value.length; i++) {
          const item = templateProcess.value[i];
          if (item.id === topoNode.processId) {
            findItem = item;
            findIndex = i;
            break;
          }
        }
        if (findItem) {
          topoNode.topoChecked = false;
          templateProcess.value.splice(findIndex as number, 1, findItem);
        } else {
          templateProcess.value.push({
            __IS_RECOVER: false,
            id: topoNode.processId!,
            topoName: topoNode.topoName,
            topoParentName: topoNode.topoParentName,
            topoNode,
          });
        }
      } else {
        // 取消选择
        const index = templateProcess.value.findIndex((item) => item.id === topoNode.processId);
        templateProcess.value.splice(index, 1);
      }
    } else if (topoNode.topoType === 'instanceProcess') {
      if (topoNode.topoChecked) {
        // 选择进程
        instanceProcess.value.push({
          __IS_RECOVER: false,
          id: topoNode.processId!,
          topoName: topoNode.topoName,
          topoParentName: topoNode.topoParentName,
          topoNode,
        });
      } else {
        // 取消选择
        const index = instanceProcess.value.findIndex((item) => item.id === topoNode.processId);
        instanceProcess.value.splice(index, 1);
      }
    }
    emits('change', {
      cc_template_process_ids: templateProcess.value.map((item) => item.id),
      cc_process_ids: instanceProcess.value.map((item) => item.id),
    });
  };

  // 移除模板进程
  const removeTemplateProcess = (item: IProcessPreviewItem, index: number) => {
    if (item.topoNode) {
      item.topoNode.topoChecked = false;
    }
    templateProcess.value.splice(index, 1);
    emits('change', {
      cc_template_process_ids: templateProcess.value.map((item) => item.id),
      cc_process_ids: instanceProcess.value.map((item) => item.id),
    });
  };
  // 移除实例进程
  const removeInstanceProcess = (item: IProcessPreviewItem, index: number) => {
    if (item.topoNode) {
      item.topoNode.topoChecked = false;
    }
    instanceProcess.value.splice(index, 1);
    emits('change', {
      cc_template_process_ids: templateProcess.value.map((item) => item.id),
      cc_process_ids: instanceProcess.value.map((item) => item.id),
    });
  };

  // 搜索树节点
  const handleSearch = (keyword: string) => {
    isSearchEmpty.value = !!keyword;
    treeLoading.value = true;
    searchTimer.value && clearTimeout(searchTimer.value);
    searchTimer.value = setTimeout(() => {
      searchTree(processType.value === 'by_topo' ? topoTreeData.value : templateTreeData.value, keyword);
      treeLoading.value = false;
    }, 300);
  };
  const searchTree = (list: ITopoTreeNode[], keyword: string) => {
    list.forEach((item) => {
      item.topoExpand = false;
      const isMatched = item.topoName.includes(keyword);
      item.topoVisible = isMatched;
      if (isMatched) {
        setParentVisible(item.topoParent as ITopoTreeNode, keyword);
      }
      if (item.child.length) {
        searchTree(item.child, keyword);
      }
    });
  };
  const setParentVisible = (node: ITopoTreeNode, keyword: string) => {
    if (node) {
      node.topoVisible = true;
      node.topoExpand = Boolean(keyword);
      setParentVisible(node.topoParent as ITopoTreeNode, keyword);
    }
  };

  const handleClearSearch = () => {
    searchValue.value = '';
    handleSearch(searchValue.value);
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    display: flex;
    width: 100%;
    height: 500px;
    .associated-wrap,
    .preview-wrap {
      padding: 20px 24px;
      width: 50%;
      height: 100%;
    }
  }
  .associated-wrap {
    .header {
      display: flex;
      align-items: center;
      gap: 12px;
      color: #979ba5;
      margin-bottom: 20px;
      .title {
        font-size: 16px;
        color: #313238;
        line-height: 24px;
      }
      .line {
        width: 1px;
        height: 16px;
        background: #dcdee5;
      }
    }
    .associated-content {
      display: flex;
      flex-direction: column;
      gap: 12px;
      height: calc(100% - 24px);
      .label {
        font-size: 14px;
        color: #4d4f56;
        line-height: 22px;
      }
      .tree-loading {
        height: 100%;
        flex: 1;
        overflow: auto;
      }
    }
  }

  .preview-wrap {
    background-color: #f5f6fa;
    .title {
      font-size: 16px;
      color: #313238;
      line-height: 24px;
    }
    .scroll-container {
      height: calc(100% - 20px);
      overflow: auto;
    }
    .exception-wrap-item {
      height: calc(100% - 20px);
      padding-top: 100px;
    }
  }
</style>
