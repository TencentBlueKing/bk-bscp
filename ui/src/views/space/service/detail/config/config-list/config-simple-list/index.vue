<template>
  <ConfigTableWithTemplate
    v-if="isFileType"
    :bk-biz-id="bkBizId"
    :project-id="projectId"
    :env-id="envId"
    :app-id="appId" />
  <ConfigTableKvWithTemplate
    v-else
    :bk-biz-id="bkBizId"
    :project-id="projectId"
    :env-id="envId"
    :app-id="appId" />
</template>

<script lang="ts" setup>
  import useServiceStore from '../../../../../../../store/service';
  import { storeToRefs } from 'pinia';
  import ConfigTableWithTemplate from './config-list-with-templates.vue';
  import ConfigTableKvWithTemplate from './config-list-kv-with-template.vue';
  const serviceStore = storeToRefs(useServiceStore());
  const { isFileType } = serviceStore;
  defineProps<{
    bkBizId: string;
    projectId: string;
    envId: string;
    appId: number;
  }>();
</script>

<style scoped lang="scss"></style>
