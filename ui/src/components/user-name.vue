<template>
  <bk-user-display-name v-if="spaceFeatureFlags.ENABLE_MULTI_TENANT_MODE" :user-id="name"></bk-user-display-name>
  <span v-else>{{ name }}</span>
</template>

<script lang="ts" setup>
  import useGlobalStore from '../store/global';
  import { storeToRefs } from 'pinia';

  const { spaceFeatureFlags } = storeToRefs(useGlobalStore());

  defineProps<{
    name: string
  }>();
</script>

<style scoped lang="scss"></style>
