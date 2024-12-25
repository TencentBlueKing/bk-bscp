<template>
  <DetailLayout :name="$t('编辑数据')" @close="handleClose">
    <template #content>
      <div class="content-wrap">
        <div class="content-header">
          <bk-button @click="isShowImportTable = true">{{ $t('导入') }}</bk-button>
          <bk-input class="search-input">
            <template #suffix>
              <Search class="search-input-icon" />
            </template>
          </bk-input>
        </div>
        <bk-loading class="loading-wrapper" :loading="loading">
          <Table :fields="fields" :data="tableData" @change="handleChange" />
        </bk-loading>
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary" style="width: 88px">{{ $t('保存') }}</bk-button>
        <bk-button style="width: 88px" @click="handleClose">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
  <ImportTable v-model:show="isShowImportTable" />
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { Search } from 'bkui-vue/lib/icon';
  import { IFiledItem } from '../../../../../../types/kv-table';
  import { getTableStructureData, getTableStructureFields } from '../../../../../api/kv-table';
  import DetailLayout from '../../component/detail-layout.vue';
  import Table from './table.vue';
  import ImportTable from './import-table.vue';

  const props = defineProps<{
    bkBizId: string;
    id: number;
  }>();

  const emits = defineEmits(['close', 'refresh']);

  const isShowImportTable = ref(false);
  const fields = ref<IFiledItem[]>([]);
  const tableData = ref<any[]>([]);
  const loading = ref(false);

  onMounted(() => {
    getFieldsList();
  });

  const getFieldsList = async () => {
    try {
      loading.value = true;
      const [fieldsData, Contentdata] = await Promise.all([
        getTableStructureFields(props.bkBizId, props.id),
        getTableStructureData(props.bkBizId, props.id),
      ]);
      fields.value = fieldsData.details.spec.columns;
      tableData.value = Contentdata.details;
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleChange = (data: any) => {
    console.log(data);
  };

  const handleClose = () => {
    emits('close');
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    background: #f5f7fa;
    padding: 24px;
    min-height: 100%;
    .content-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 16px;
      .search-input {
        width: 600px;
      }
    }
  }

  .operation-btns {
    width: calc(100% - 48px);
    display: flex;
    gap: 8px;
  }

  .loading-wrapper {
    height: calc(100% - 49px);
  }
</style>
