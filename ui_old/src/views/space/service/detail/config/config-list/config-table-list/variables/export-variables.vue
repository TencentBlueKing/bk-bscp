<template>
  <bk-popover
    ref="buttonRef"
    :arrow="false"
    placement="bottom-start"
    theme="light export-config-button-popover"
    trigger="click"
    @after-show="isExportPopoverShow = true"
    @after-hidden="isExportPopoverShow = false">
    <bk-button :disabled="disabled">{{ $t('导出至') }}</bk-button>
    <template #content>
      <div class="export-config-operations">
        <div v-for="item in exportItem" :key="item.value" class="operation-item" @click="handleExport(item.value)">
          <span :class="['bk-bscp-icon', `icon-${item.value}`]" />
          <span class="text"> {{ item.text }}</span>
        </div>
      </div>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';

  withDefaults(
    defineProps<{
      disabled?: boolean;
    }>(),
    {
      disabled: false,
    },
  );

  const emits = defineEmits(['export']);

  const isExportPopoverShow = ref(false);
  const buttonRef = ref();
  const exportItem = [
    {
      text: 'JSON',
      value: 'json',
    },
    {
      text: 'YAML',
      value: 'yaml',
    },
  ];

  // 导出变量
  const handleExport = async (type: string) => {
    emits('export', type);
    buttonRef.value.hide();
  };
</script>

<style lang="scss">
  .export-config-button-popover.bk-popover.bk-pop2-content {
    padding: 4px 0;
    border: 1px solid #dcdee5;
    box-shadow: 0 2px 6px 0 #0000001a;
    .export-config-operations {
      .operation-item {
        padding: 0 12px;
        min-width: 100px;
        height: 32px;
        line-height: 32px;
        color: #63656e;
        font-size: 14px;
        align-items: center;
        cursor: pointer;
        &:hover {
          background: #f5f7fa;
        }
        .text {
          margin-left: 4px;
          font-size: 12px;
        }
      }
    }
  }
</style>
