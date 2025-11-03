<template>
  <DetailLayout :name="$t('任务详情')" :show-footer="false" @close="handleClose">
    <template #header-suffix>
      <bk-tag type="filled" theme="info">正在执行</bk-tag>
    </template>
    <template #content>
      <div class="detail-content">
        <Info />
        <DetailList />
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import Info from './info.vue';
  import DetailList from './detail-list.vue';
  const handleClose = () => {};
</script>

<style scoped lang="scss">
  .detail-content {
    background: #f5f7fa;
    height: 100%;
  }
</style>
