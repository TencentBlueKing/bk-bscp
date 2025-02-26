<template>
  <div class="card">
    <div class="card-header">
      <span class="title">{{ title }}</span>
      <slot name="suffix" />
    </div>
    <slot />
  </div>
</template>

<script lang="ts" setup>
  defineProps<{
    title: string;
  }>();
</script>

<style scoped lang="scss">
  .card {
    padding: 16px 24px 24px;
    width: 1000px;
    background: #ffffff;
    box-shadow: 0 2px 4px 0 #1919290d;
    .card-header {
      display: flex;
      align-items: center;
      margin-bottom: 16px;
      .title {
        margin-right: 16px;
        font-size: 14px;
        color: #63656e;
        font-weight: 700;
      }
    }
  }
</style>
