<template>
  <div class="custom-tree-select-container">
    <bk-popover
      ref="popoverRef"
      theme="light custom-tree-select"
      trigger="click"
      placement="bottom"
      :arrow="false"
      width="260"
      @after-show="handleDropdownShow"
      @after-hidden="handleDropdownHide">
      <div ref="triggerRef" class="custom-tree-select-trigger" :class="{ active: showDropdown }">
        <span class="select-prefix">{{ $t('进程实例') }}</span>
        <div class="search-select">
          <span v-if="selectedName">{{ selectedName }}</span>
          <span v-else class="holder-text">{{ $t('请选择') }}</span>
          <angle-down class="icon-angle-down" />
        </div>
      </div>
      <template #content>
        <div ref="contentRef" class="custom-tree-select-content">
          <div class="input-container">
            <search class="icon-search" />
            <input
              v-model="searchKeyword"
              type="text"
              class="search-input"
              :placeholder="$t('请输入关键字')"
              @input="handleSearch" />
          </div>
          <div class="tree-container">
            <Tree :node-list="treeData" @checked="handleCheckNode" />
          </div>
        </div>
      </template>
    </bk-popover>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { AngleDown, Search } from 'bkui-vue/lib/icon';
  import Tree from './tree.vue';
  import type { ITopoTreeNode } from '../../../../../../types/config-template';

  const props = defineProps<{
    treeData: ITopoTreeNode[];
  }>();

  const emit = defineEmits(['selected']);

  const popoverRef = ref();

  const selectedName = ref('');
  const showDropdown = ref(false);
  const searchKeyword = ref('');
  const searchLoading = ref(false);
  const searchTimer = ref(0);

  const handleDropdownShow = () => {
    showDropdown.value = true;
  };

  const handleDropdownHide = () => {
    showDropdown.value = false;
  };

  // 树选择
  const handleCheckNode = (topoNode: ITopoTreeNode) => {
    setNodeChecked(topoNode, props.treeData);
    selectedName.value = topoNode.topoName;
    emit('selected', topoNode.bk_inst_id);
    popoverRef.value.hide();
  };

  // 设置选中叶子节点 checked 为 true，其他为 false
  const setNodeChecked = (checkedNode: ITopoTreeNode, nodeList: ITopoTreeNode[]) => {
    nodeList.forEach((topoNode) => {
      topoNode.topoChecked = topoNode === checkedNode;
      if (topoNode.child?.length) {
        setNodeChecked(checkedNode, topoNode.child);
      }
    });
  };

  // 树搜索
  const handleSearch = () => {
    searchLoading.value = true;
    if (searchTimer.value) clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
      searchTree(props.treeData, searchKeyword.value);
      searchLoading.value = false;
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

      if (item.child?.length) {
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
</script>

<style scoped lang="scss">
  .custom-tree-select-container {
    .custom-tree-select-trigger {
      display: flex;
      align-items: center;
      height: 26px;
      line-height: 26px;
      border-radius: 2px;
      cursor: pointer;
      font-size: 12px;
      color: #c4c6cc;
      border: 1px solid #63656e;
      width: 260px;
      margin-right: 16px;
      .search-select {
        padding: 0 8px;
        display: flex;
        justify-content: space-between;
        align-items: center;
        flex: 1;
        height: 26px;
        input {
          background: #2e2e2e;
          color: #b3b3b3;
        }
        .holder-text {
          color: #63656e;
        }

        .icon-angle-down {
          color: #979ba5;
          font-size: 20px;
          transition: transform 0.2s;
        }
      }
      .select-prefix {
        height: 100%;
        padding: 0 8px;
        background: #3d3d3d;
        color: #b3b3b3;
      }
      &:hover {
        color: #dcdee5;
        border-color: #ccc;
      }
      &.active {
        border-color: #3a84ff;
        .icon-angle-down {
          transform: rotate(-180deg);
        }
      }
    }
  }

  .custom-tree-select-content {
    padding-top: 6px;
    .input-container {
      display: flex;
      align-items: center;
      height: 32px;
      margin: 0 10px;
      border-bottom: 1px solid #63656e;

      .icon-search {
        flex-shrink: 0;
        font-size: 16px;
        color: #63656e;
        margin: 0 4px 0 5px;
      }

      .search-input {
        width: 100%;
        border: 0;
        color: #c4c6cc;
        background-color: #383838;

        &::placeholder {
          padding-left: 2px;
          color: #63656e;
        }
      }
    }

    .tree-container {
      min-height: 42px;
      max-height: 294px;
      padding-bottom: 6px;
      overflow: auto;
    }
  }
</style>

<style lang="scss">
  .custom-tree-select {
    padding: 0 !important;
    background-color: #383838 !important;
  }
</style>
