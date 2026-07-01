<template>
  <bk-dialog
    :is-show="props.show"
    :title="t('批量导入')"
    :theme="'primary'"
    width="1200"
    height="720"
    ext-cls="variable-import-dialog"
    :esc-close="false"
    @closed="handleClose">
    <div class="selector-wrap">
      <div class="selector-label">{{ $t('文本格式') }}</div>
      <div class="selector-content">
        <bk-radio-group v-model="selectFormat">
          <bk-radio label="text">{{ $t('简单文本') }}</bk-radio>
          <bk-radio label="json">JSON</bk-radio>
          <bk-radio label="yaml">YAML</bk-radio>
        </bk-radio-group>
        <div class="tips">{{ tips }}</div>
      </div>
    </div>
    <div :class="['content-wrapper', { 'show-example': isShowFormateExample }]">
      <VariableContentEditor
        ref="editorRef"
        v-model="isShowFormateExample"
        :format="selectFormat"
        @has-error="hasTextImportError = $event">
        <template #sufContent>
          <FormatExample v-if="isShowFormateExample" :format="selectFormat" />
        </template>
      </VariableContentEditor>
    </div>
    <template #footer>
      <bk-button
        theme="primary"
        style="margin-right: 8px"
        :disabled="hasTextImportError"
        :loading="loading"
        @click="handleConfirm">
        {{ t('导入') }}
      </bk-button>
      <bk-button @click="handleClose">{{ t('取消') }}</bk-button>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ref, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import VariableContentEditor from './variables-content-editor.vue';
  import FormatExample from './format-example.vue';
  import { Message } from 'bkui-vue';

  const { t } = useI18n();

  const props = defineProps<{
    show: boolean;
  }>();
  const editorRef = ref();
  const emits = defineEmits(['update:show', 'imported', 'topIds']);
  const selectFormat = ref('text');
  const isShowFormateExample = ref(true);
  const hasTextImportError = ref(false);
  const loading = ref(false);

  const tips = computed(() => {
    if (selectFormat.value === 'text') {
      return t('每行表示一个变量，包含变量名称、变量类型、默认值与描述（可选），默认使用空格分隔');
    }
    if (selectFormat.value === 'json') {
      return t(
        '以 JSON 格式批量导入变量，变量名称作为 JSON 对象的 Key，而变量类型、默认值、描述组成一个嵌套对象，作为对应 Key 的 Value',
      );
    }
    return t(
      '以 YAML 格式批量导入变量，变量名称作为 YAML 对象的 Key，而变量类型、默认值、描述分别作为嵌套对象的子键，形成对应键的值 ',
    );
  });

  const handleClose = () => {
    emits('update:show', false);
  };
  const handleConfirm = async () => {
    try {
      loading.value = true;
      const topIds = await editorRef.value.handleImport();
      emits('topIds', topIds);
      emits('imported');
      emits('update:show', false);
      Message({
        theme: 'success',
        message: t('变量导入成功'),
      });
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };
</script>

<style scoped lang="scss">
  .selector-wrap {
    display: flex;
    font-size: 12px;
    align-items: flex-start;
    .selector-label {
      margin: 0 16px;
    }
    .tips {
      font-size: 12px;
      color: #979ba5;
      &.en-tips {
        margin-left: 122px;
      }
    }
    :deep(.bk-radio-label) {
      font-size: 12px;
    }
  }
  .content-wrapper {
    width: 100%;
    margin-top: 24px;
    &.show-example {
      :deep(.config-content-editor) {
        height: 484px;
        .code-editor-wrapper {
          width: calc(100% - 520px);
        }
      }
    }
  }
  :deep(.editor-content) {
    display: flex;
    .code-editor-wrapper {
      width: 100%;
    }
  }
</style>

<style lang="scss">
  .variable-import-dialog {
    .bk-modal-content {
      overflow: hidden !important;
    }
  }
</style>
