<template>
  <div class="config-list-area">
    <ConfigSimpleList
      v-if="versionDetailView"
      :bk-biz-id="props.bkBizId"
      :project-id="props.projectId"
      :env-id="props.envId"
      :app-id="props.appId" />
    <ConfigTableList
      v-else
      :bk-biz-id="props.bkBizId"
      :project-id="props.projectId"
      :env-id="props.envId"
      :app-id="props.appId" />
  </div>
</template>
<script setup lang="ts">
  import ConfigSimpleList from './config-simple-list/index.vue';
  // vue/html-closing-bracket-newline
  import ConfigTableList from './config-table-list/index.vue';

  const props = defineProps<{
    versionDetailView: boolean;
    bkBizId: string;
    projectId: string;
    envId: string;
    appId: number;
  }>();
</script>
<style lang="scss" scoped>
  .config-list-area {
    height: 100%;
  }
</style>
