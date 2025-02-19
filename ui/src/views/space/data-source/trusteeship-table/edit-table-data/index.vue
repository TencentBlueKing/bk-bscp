<template>
  <DetailLayout :name="$t('编辑数据')" :suffix="name" @close="handleClose">
    <template #content>
      <div class="content-wrap">
        <div class="content-header">
          <div class="operation-btns">
            <bk-button @click="handleAddData">{{ $t('新增') }}</bk-button>
            <bk-button @click="isShowImportTable = true">{{ $t('导入') }}</bk-button>
          </div>
          <bk-input class="search-input">
            <template #suffix>
              <Search class="search-input-icon" />
            </template>
          </bk-input>
        </div>
        <bk-loading class="loading-wrapper" :loading="loading">
          <Table ref="tableRef" :fields="fields" :data="tableData" @change="handleChange" />
        </bk-loading>
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button
          :loading="confirmLoading"
          theme="primary"
          style="width: 88px"
          @click="handleConfirm">
          {{ $t('保存') }}
        </bk-button>
        <bk-button style="width: 88px" @click="handleClose">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
  <ImportTable v-model:show="isShowImportTable" :bk-biz-id="spaceId" :id="tableId"/>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { Search } from 'bkui-vue/lib/icon';
  import {
    IFieldItem,
    ILocalTableEditData,
    ILocalTableDataItem,
    ILocalTableEditQuery,
  } from '../../../../../../types/kv-table';
  import { getTableData, getTableStructure, editTableData } from '../../../../../api/kv-table';
  import DetailLayout from '../../component/detail-layout.vue';
  import Table from './table.vue';
  import ImportTable from './import-table.vue';
  import BkMessage from 'bkui-vue/lib/message';
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();

  const router = useRouter();
  const route = useRoute();

  const tableId = ref(Number(route.params.id));
  const spaceId = ref(String(route.params.spaceId));

  const name = ref(String(route.query.name));

  const isShowImportTable = ref(false);
  const fields = ref<IFieldItem[]>([]);
  const tableData = ref<ILocalTableDataItem[]>([]);
  const editDataContent = ref<ILocalTableEditQuery>([]);
  const loading = ref(false);
  const confirmLoading = ref(false);
  const tableRef = ref();

  onMounted(() => {
    getFieldsList();
  });

  const getFieldsList = async () => {
    try {
      loading.value = true;
      const query = {
        start: 0,
        all: true,
      };
      const [fieldsData, Contentdata] = await Promise.all([
        getTableStructure(spaceId.value, tableId.value),
        getTableData(spaceId.value, tableId.value, query),
      ]);
      fields.value = fieldsData.details.spec.columns;
      tableData.value = Contentdata.details;
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleConfirm = async () => {
    try {
      const validate = await tableRef.value.fullValidEvent();
      if (!validate) return;
      confirmLoading.value = true;
      await editTableData(spaceId.value, tableId.value, {
        contents: editDataContent.value,
      });
      handleClose();
      BkMessage({ theme: 'success', message: t('编辑数据成功') });
    } catch (error) {
      console.error(error);
    } finally {
      confirmLoading.value = false;
    }
  };

  const handleAddData = () => {
    tableRef.value.handleAddData();
  };

  const handleChange = (data: ILocalTableEditData[]) => {
    editDataContent.value = data.map((item) => item.content);
  };

  const handleClose = () => {
    router.push({ name: 'trusteeship-table-list', params: { spaceId: spaceId.value } });
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    background: #f5f7fa;
    padding: 24px;
    min-height: 100%;
    .content-header {
      .operation-btns {
        display: flex;
        gap: 8px;
      }
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

  .search-input-icon {
    padding-right: 10px;
    font-size: 16px;
    color: #979ba5;
    background: #ffffff;
  }
</style>
