<template>
  <bk-table :data="tableData" :border="['outer']">
    <bk-table-column v-for="item in tableColumns" :key="item" :label="item" :prop="item"></bk-table-column>
  </bk-table>
</template>

<script lang="ts" setup>
  import { computed } from 'vue';
  // defineProps<{
  //   data: any[];
  // }>();

  const tableData = [
    {
      name: '张三',
      age: 18,
      gender: '男',
    },
    {
      name: '李四',
      age: 20,
      gender: '女',
    },
  ];

  const tableColumns = computed(() => {
    return Object.keys(tableData[0]);
  });
</script>

<style scoped lang="scss"></style>
