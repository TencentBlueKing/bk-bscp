<template>
  <section class="code-editor-wrapper" ref="codeEditorRef"></section>
</template>
<script setup lang="ts">
  import { ref, watch, onMounted, computed } from 'vue';
  import * as monaco from 'monaco-editor';
  import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker.js?worker';

  let editor: monaco.editor.IStandaloneCodeEditor;

  const props = defineProps<{
    modelValue: string;
    isCipher: boolean;
    editable: boolean;
  }>();

  const emits = defineEmits(['update:modelValue']);

  const codeEditorRef = ref();
  const localVal = ref(props.modelValue);

  const cipherText = computed(() => localVal.value.replace(/./g, '*'));

  self.MonacoEnvironment = {
    getWorker() {
      return new editorWorker();
    },
  };

  watch(
    () => props.modelValue,
    (val) => {
      localVal.value = val;
      if (!props.isCipher) {
        editor.setValue(val);
      } else {
        editor.setValue(cipherText.value);
      }
    },
  );

  watch(
    () => props.isCipher,
    (val) => {
      if (val) {
        editor.setValue(cipherText.value);
      } else {
        editor.setValue(localVal.value);
      }
    },
  );

  onMounted(() => {
    if (!editor) {
      editor = monaco.editor.create(codeEditorRef.value as HTMLElement, {
        value: props.isCipher ? cipherText.value : localVal.value,
        theme: 'vs-dark',
        automaticLayout: true,
        language: 'plaintext',
        readOnly: !props.editable,
      });
    }

    editor.onDidChangeModelContent(() => {
      if (props.editable) {
        localVal.value = editor.getValue();
        emits('update:modelValue', localVal.value);
      }
    });
  });

  const destroy = () => {
    if (editor) {
      editor.dispose();
    }
  };

  defineExpose({
    destroy,
  });
</script>
<style lang="scss" scoped>
  .code-editor-wrapper {
    height: 100%;
    :deep(.monaco-editor) {
      width: 100%;
      .template-variable-item {
        color: #1768ef;
        border: 1px solid #1768ef;
        cursor: pointer;
      }
    }
  }
</style>
