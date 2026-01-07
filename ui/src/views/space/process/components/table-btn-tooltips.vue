<template>
  <bk-popover placement="top" :disabled="disabled">
    <slot></slot>
    <template #content>
      <span v-if="reason === 'CMD_NOT_CONFIGURED'" class="no-cmd-content" @click="handleGoBKCC">
        {{ $t('尚未配置操作命令') }}
        <span class="primary">
          {{ $t('前往 BKCC 配置') }}
          <Share />
        </span>
      </span>
      <span v-else>
        {{ PROCESS_BUTTON_DISABLED_TIPS[reason as keyof typeof PROCESS_BUTTON_DISABLED_TIPS] }}
      </span>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { PROCESS_BUTTON_DISABLED_TIPS } from '../../../../constants/process';

  const props = defineProps<{
    disabled: boolean;
    reason: string;
    link: string;
  }>();

  const handleGoBKCC = () => {
    window.open(props.link);
  };
</script>

<style scoped lang="scss"></style>
