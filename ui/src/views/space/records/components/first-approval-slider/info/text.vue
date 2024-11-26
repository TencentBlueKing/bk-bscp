<template>
  <div class="info-wrapper">
    <section ref="fillInfoRef" class="fill-info-wrapper"></section>
  </div>
</template>
<script setup lang="ts">
  import { ref, watch, onMounted, onBeforeUnmount, computed } from 'vue';
  import * as monaco from 'monaco-editor';
  import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker.js?worker';
  import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker.js?worker';
  import cssWorker from 'monaco-editor/esm/vs/language/css/css.worker.js?worker';
  import htmlWorker from 'monaco-editor/esm/vs/language/html/html.worker.js?worker';
  import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker.js?worker';
  import { IVariableEditParams } from '../../../../../../../types/variable';

  self.MonacoEnvironment = {
    getWorker(_, label) {
      if (label === 'json') {
        return new jsonWorker();
      }
      if (label === 'css' || label === 'scss' || label === 'less') {
        return new cssWorker();
      }
      if (label === 'html' || label === 'handlebars' || label === 'razor') {
        return new htmlWorker();
      }
      if (label === 'typescript' || label === 'javascript') {
        return new tsWorker();
      }
      return new editorWorker();
    },
  };

  const props = withDefaults(
    defineProps<{
      current: string;
      currentLanguage?: string;
      currentVariables?: IVariableEditParams[];
      currentPermission?: string;
      isSecret?: boolean;
      secretVisible?: boolean;
      isCipherShow?: boolean;
    }>(),
    {
      currentVariables: () => [],
      currentLanguage: '',
      isSecret: false,
      secretVisible: true,
    },
  );

  const fillInfoRef = ref();
  const cipherText = '********';
  let codeEditor: monaco.editor.IStandaloneCodeEditor;

  const currentShowContent = computed(() => {
    if (props.isSecret && props.isCipherShow) {
      return cipherText;
    }
    return props.current;
  });

  watch(
    () => props.current,
    () => {
      updateModel();
    },
  );

  watch(
    () => props.currentLanguage,
    () => {
      updateModel();
    },
  );

  watch(
    () => props.currentPermission,
    () => {
      updateModel();
    },
  );

  watch(
    () => props.isCipherShow,
    () => {
      updateModel();
    },
  );

  onMounted(() => {
    createDiffEditor();
  });

  onBeforeUnmount(() => {
    codeEditor.dispose();
  });

  const createDiffEditor = () => {
    if (codeEditor) {
      codeEditor.dispose();
    }
    const modifiedModel = monaco.editor.createModel(currentShowContent.value, props.currentLanguage);
    codeEditor = monaco.editor.create(fillInfoRef.value, {
      theme: 'vs-dark',
      automaticLayout: true,
      scrollBeyondLastLine: false,
      readOnly: true,
      unicodeHighlight: {
        ambiguousCharacters: false,
      },
    });
    codeEditor.setModel(modifiedModel);
  };

  const updateModel = () => {
    const modifiedModel = monaco.editor.createModel(currentShowContent.value, props.currentLanguage);
    codeEditor.setModel(modifiedModel);
  };
</script>
<style lang="scss" scoped>
  .fill-info-wrapper {
    flex: 1;
    overflow: hidden;
    border-top: 2px solid #2c2c2c;
  }
  :deep(.d2h-file-wrapper) {
    border: none;
  }

  .info-wrapper {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
</style>
