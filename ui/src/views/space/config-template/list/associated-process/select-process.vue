<template>
  <div class="associated-wrap">
    <div class="header">
      <span class="title">
        {{ $t('关联进程实例') }}
      </span>
      <span class="line"></span>
      <span>模板文件 1 (/tmp/file1.ini)</span>
    </div>
    <div class="associated-content">
      <div class="label">{{ $t('选择关联进程') }}</div>
      <bk-radio-group v-model="processType" type="card" @change="loadProcessTree">
        <bk-radio-button label="by_topo">{{ $t('按业务拓扑') }}</bk-radio-button>
        <bk-radio-button label="by_service">{{ $t('按服务模版') }}</bk-radio-button>
      </bk-radio-group>
      <SearchInput v-model="searchValue" class="search-input" />
      <ProcessTree class="process-tree" :tree="processTreeData" />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { getProcessTree } from '../../../../../api/config-template';
  import type { IProcessTreeNode } from '../../../../../../types/config-template';
  import SearchInput from '../../../../../components/search-input.vue';
  import ProcessTree from './process-tree.vue';

  const props = defineProps<{
    bkBizId: string;
  }>();

  const processType = ref('by_topo');
  const searchValue = ref('');
  const processTreeData = ref<IProcessTreeNode[]>([]);

  onMounted(() => {
    loadProcessTree();
  });

  const loadProcessTree = async () => {
    try {
      const res = await getProcessTree(props.bkBizId, processType.value);
      processTreeData.value = res.topology;
    } catch (error) {
      console.error(error);
    }
  };
</script>

<style scoped lang="scss">
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
      height: 100%;

      .label {
        font-size: 14px;
        color: #4d4f56;
        line-height: 22px;
      }
      .process-tree {
        flex: 1;
        overflow: auto;
      }
    }
  }
</style>
