<template>
  <section class="process-page">
    <Table></Table>
  </section>
</template>

<script lang="ts" setup>
  import Table from './components/table.vue';
</script>

<style scoped lang="scss">
  .process-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 24px;
    background: #f5f7fa;
  }
</style>
