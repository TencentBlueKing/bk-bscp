<template>
  <DetailLayout :name="$t('编辑表结构')" @close="handleClose">
    <template #content>
      <div class="content-wrap">
        <bk-loading :loading="formLoading">
          <TableStructureForm
            ref="formRef"
            :bk-biz-id="spaceId"
            :form="formData"
            :is-manual-create="true"
            :is-edit="true"
            :has-table-data="hasTableData"
            @change="handleFormChange" />
        </bk-loading>
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary" :loading="loading" style="width: 88px" @click="handleConfirm">
          {{ $t('创建') }}
        </bk-button>
        <bk-button :loading="loading" style="width: 130px">{{ $t('创建并编辑数据') }}</bk-button>
        <bk-button style="width: 88px" @click="handleClose">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { onMounted, ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { getTableStructure, editTable, getTableStructureHasData } from '../../../../../api/kv-table';
  import { ILocalTableForm } from '../../../../../../types/kv-table';
  import DetailLayout from '../../component/detail-layout.vue';
  import TableStructureForm from '../components/table-structure-form.vue';

  const router = useRouter();
  const route = useRoute();

  const tableId = ref(Number(route.params.id));
  const spaceId = ref(String(route.params.spaceId));

  const formData = ref<ILocalTableForm>({
    table_name: '',
    table_memo: '',
    visible_range: [],
    columns: [],
  });
  const loading = ref(false);
  const formLoading = ref(false);
  const formRef = ref();
  const hasTableData = ref(false);

  onMounted(async () => {
    await getStructureData();
    formRef.value.translateFormData();
  });

  const getStructureData = async () => {
    try {
      formLoading.value = true;
      const [data, hasData] = await Promise.all([
        getTableStructure(spaceId.value, tableId.value),
        getTableStructureHasData(spaceId.value, tableId.value),
      ]);
      formData.value = data.details.spec;
      hasTableData.value = hasData.exist;
      formRef.value.translateFormData();
    } catch (error) {
      console.error(error);
    } finally {
      formLoading.value = false;
    }
  };

  const handleConfirm = async () => {
    try {
      loading.value = true;
      const data = {
        spec: formData.value,
      };
      await editTable(spaceId.value, tableId.value, data);
      handleClose();
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleFormChange = (form: ILocalTableForm) => {
    formData.value = form;
  };

  const handleClose = () => {
    router.push({ name: 'trusteeship-table-list', params: { spaceId: spaceId.value } });
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    background: #f5f7fa;
    padding: 24px 0;
    min-height: 100%;
  }

  .operation-btns {
    height: 100%;
    width: 1000px;
    display: flex;
    gap: 8px;
  }
</style>
