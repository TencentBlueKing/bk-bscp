<template>
  <DetailLayout :name="$t('编辑表结构')" @close="handleClose">
    <template #content>
      <div class="content-wrap">
        <TableStructureForm :bk-biz-id="bkBizId" :is-manual-create="true" :is-edit="true"/>
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary" style="width: 88px">{{ $t('创建') }}</bk-button>
        <bk-button style="width: 130px">{{ $t('创建并编辑数据') }}</bk-button>
        <bk-button style="width: 88px" @click="handleClose">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import DetailLayout from '../component/detail-layout.vue';
  import TableStructureForm from './components/table-structure-form.vue';

  defineProps<{
    bkBizId: string;
  }>();

  const emits = defineEmits(['close']);

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
    display: flex;
    gap: 8px;
  }
</style>
