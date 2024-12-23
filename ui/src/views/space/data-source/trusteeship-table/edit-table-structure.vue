<template>
  <DetailLayout :name="$t('编辑表结构')" @close="handleClose">
    <template #content>
      <div class="content-wrap">
        <bk-loading :loading="formLoading">
          <TableStructureForm
            :bk-biz-id="bkBizId"
            :form="formData"
            :is-manual-create="true"
            :is-edit="true"
            @change="formData = $event" />
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
  import { getTableStructure, editTableStructure } from '../../../../api/kv-table';
  import { ILocalTableForm } from '../../../../../types/kv-table';
  import DetailLayout from '../component/detail-layout.vue';
  import TableStructureForm from './components/table-structure-form.vue';

  const props = defineProps<{
    bkBizId: string;
    id: number;
  }>();

  const emits = defineEmits(['refresh', 'close']);

  const formData = ref<ILocalTableForm>({
    table_name: '',
    table_memo: '',
    visible_range: [],
    columns: [],
  });
  const loading = ref(false);
  const formLoading = ref(false);

  onMounted(() => {
    getStructureData();
  });

  const getStructureData = async () => {
    try {
      formLoading.value = true;
      const res = await getTableStructure(props.bkBizId, props.id);
      formData.value = res.details.spec;
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
      await editTableStructure(props.bkBizId, props.id, JSON.stringify(data));
      emits('close');
      emits('refresh');
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleClose = () => {
    emits('close');
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
