<template>
  <section class="version-config-content">
    <ConfigList
      :version-detail-view="versionDetailView"
      :bk-biz-id="props.bkBizId"
      :project-id="projectId"
      :env-id="envId"
      :app-id="props.appId" />
  </section>
</template>
<script setup lang="ts">
  import { storeToRefs } from 'pinia';
  import useConfigStore from '../../../../../store/config';
  import ConfigList from './config-list/index.vue';

  const { versionDetailView } = storeToRefs(useConfigStore());

  const props = defineProps<{
    bkBizId: string;
    projectId: string;
    envId: string;
    appId: number;
  }>();
</script>
<style lang="scss" scoped>
  .version-config-content {
    height: 100%;
    background: #ffffff;
  }
</style>
