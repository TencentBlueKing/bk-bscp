<template>
  <div class="version-list-aside">
    <VersionTableList
      v-if="versionDetailView"
      :bk-biz-id="props.bkBizId"
      :project-id="props.projectId"
      :env-id="props.envId"
      :app-id="props.appId" />
    <VersionSimpleList
      v-else
      :bk-biz-id="props.bkBizId"
      :project-id="props.projectId"
      :env-id="props.envId"
      :app-id="props.appId" />
  </div>
</template>
<script setup lang="ts">
  import VersionSimpleList from './version-simple-list.vue';
  import VersionTableList from './version-table-list.vue';

  const props = defineProps<{
    versionDetailView: Boolean;
    bkBizId: string;
    projectId: string;
    envId: string;
    appId: number;
  }>();
</script>
<style lang="scss" scoped>
  .version-list-aside {
    position: relative;
    height: 100%;
    background: #fafbfd;
  }
</style>
