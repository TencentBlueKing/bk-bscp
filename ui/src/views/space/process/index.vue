<template>
  <section class="process-page">
    <div class="status-and-screen">
      <SyncStatus />
      <Screen />
    </div>
    <Table></Table>
  </section>
</template>

<script lang="ts" setup>
  import SyncStatus from './components/sync-status.vue';
  import Screen from './components/screen.vue';
  import Table from './components/table.vue';
</script>

<style scoped lang="scss">
  .process-page {
    height: 100%;
    padding: 24px;
    background: #f5f7fa;
    .status-and-screen {
      display: flex;
      align-items: center;
      justify-content: space-between;
    margin-bottom: 16px;

    }
  }
</style>
