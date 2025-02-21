<template>
  <DetailLayout :name="$t('编辑表结构')" @close="handleClose">
    <template #content>
      <div class="content-wrap">
        <bk-loading :loading="formLoading">
          <ManualCreate
            ref="fieldRef"
            :columns="fieldsColumns"
            :is-manual-create="true"
            :bk-biz-id="spaceId"
            :is-edit="false"
            @change="fieldsColumns = $event" />
          <baseInfoForm
            ref="formRef"
            :bk-biz-id="spaceId"
            :is-edit="false"
            :form="formData"
            @change="formData = $event" />
        </bk-loading>
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary" :loading="loading" style="width: 88px" @click="handleConfirm(false)">
          {{ $t('创建') }}
        </bk-button>
        <bk-button :loading="loading" style="width: 130px" @click="handleConfirm(true)">
          {{ $t('创建并编辑数据') }}
        </bk-button>
        <bk-button style="width: 88px" @click="handleClose">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { onMounted, ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { getTableStructure, editTable, getTableStructureHasData } from '../../../../../api/kv-table';
  import { IFieldItem, ILocalTableBase } from '../../../../../../types/kv-table';
  import DetailLayout from '../../component/detail-layout.vue';
  import ManualCreate from '../create-table/manual-create.vue';
  import BkMessage from 'bkui-vue/lib/message';
  import baseInfoForm from '../components/base-info-form.vue';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const router = useRouter();
  const route = useRoute();

  const tableId = ref(Number(route.params.id));
  const spaceId = ref(String(route.params.spaceId));

  const formData = ref<ILocalTableBase>({
    table_name: '',
    table_memo: '',
    visible_range: ['*'],
  });

  const fieldsColumns = ref<IFieldItem[]>([]);
  const loading = ref(false);
  const formLoading = ref(false);
  const formRef = ref();
  const fieldRef = ref();
  const hasTableData = ref(false);

  onMounted(async () => {
    await getStructureData();
  });

  const getStructureData = async () => {
    try {
      formLoading.value = true;
      const [data, hasData] = await Promise.all([
        getTableStructure(spaceId.value, tableId.value),
        getTableStructureHasData(spaceId.value, tableId.value),
      ]);
      const { columns, ...rest } = data.details.spec;
      formData.value = rest;
      fieldsColumns.value = columns;
      hasTableData.value = hasData.exist;
    } catch (error) {
      console.error(error);
    } finally {
      formLoading.value = false;
    }
  };

  const handleConfirm = async (redirectToEdit = false) => {
    try {
      const validate = (await formRef.value.validate()) && (await fieldRef.value.validate());
      if (!validate) return;
      loading.value = true;
      const data = {
        spec: {
          ...formData.value,
          columns: fieldsColumns.value,
        },
      };

      const res = await editTable(spaceId.value, tableId.value, data);

      if (redirectToEdit) {
        // 跳转到编辑页面
        router.push({
          name: 'edit-table-data',
          params: { spaceId: spaceId.value, id: res.data.id },
          query: { name: formData.value.table_name },
        });
      } else {
        // 关闭弹窗
        handleClose();
      }
      BkMessage({ theme: 'success', message: t('编辑表格成功') });
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
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
