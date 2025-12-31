<template>
  <Teleport :disabled="!isOpenFullScreen" to="body">
    <div :class="['content', { fullscreen: isOpenFullScreen }]">
      <div class="config-content-editor">
        <div class="editor-title">
          <span>{{ t('配置内容') }}</span>
          <div class="btns">
            <bk-select class="highlight-select" v-model="highlight" :filterable="false" :clearable="false">
              <template #prefix>
                <span class="select-prefix">{{ t('高亮风格') }}</span>
              </template>
              <bk-option v-for="(item, index) in highlightOptions" :id="item" :key="index" :name="item" />
            </bk-select>
            <FilliscreenLine
              v-if="!isOpenFullScreen"
              v-bk-tooltips="{
                content: t('全屏'),
                placement: 'top',
                distance: 20,
              }"
              @click="handleOpenFullScreen" />
            <UnfullScreen
              v-else
              v-bk-tooltips="{
                content: t('退出全屏'),
                placement: 'bottom',
                distance: 20,
              }"
              @click="handleCloseFullScreen" />
          </div>
        </div>
        <div class="editor-content">
          <CodeEditor
            ref="codeEditorRef"
            :model-value="props.content"
            :editable="editable"
            :language="highlight"
            @update:model-value="emits('change', $event)" />
        </div>
        <div class="editor-footer">
          <bk-button @click="suffix = 'variable'">{{ t('变量') }}</bk-button>
          <bk-button @click="handlePreview">{{ t('预览') }}</bk-button>
        </div>
      </div>
      <ProcessPreview
        v-show="suffix === 'preview'"
        ref="previewRef"
        :bk-biz-id="bkBizId"
        :config-content="props.content"
        @close="suffix = ''" />
      <Variable v-show="suffix === 'variable'" :bk-biz-id="bkBizId" @close="suffix = ''" />
    </div>
  </Teleport>
</template>

<script lang="ts" setup>
  import { ref, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { FilliscreenLine, UnfullScreen } from 'bkui-vue/lib/icon';
  import CodeEditor from '../../../../components/code-editor/index.vue';
  import BkMessage from 'bkui-vue/lib/message';
  import ProcessPreview from './process-preview/index.vue';
  import Variable from './variable.vue';

  const { t } = useI18n();

  const emits = defineEmits(['change']);
  const props = withDefaults(
    defineProps<{
      content: string;
      bkBizId: string;
      templateId?: number;
      editable?: boolean;
      charset?: string;
      sizeLimit?: number;
    }>(),
    {
      editable: true,
      sizeLimit: 100,
      language: '',
    },
  );

  const isOpenFullScreen = ref(false);
  const codeEditorRef = ref();
  const highlight = ref('python');
  const highlightOptions = ['python', 'shell', 'bat', 'powershell'];
  const suffix = ref('');
  const previewRef = ref();

  onBeforeUnmount(() => {
    codeEditorRef.value.destroy();
  });

  // 打开全屏
  const handleOpenFullScreen = () => {
    isOpenFullScreen.value = true;
    window.addEventListener('keydown', handleEscClose);
    BkMessage({
      theme: 'primary',
      message: t('按 Esc 即可退出全屏模式'),
    });
  };

  const handleCloseFullScreen = () => {
    isOpenFullScreen.value = false;
    window.removeEventListener('keydown', handleEscClose);
  };

  // Esc按键事件处理
  const handleEscClose = (event: KeyboardEvent) => {
    if (event.code === 'Escape') {
      handleCloseFullScreen();
    }
  };

  const handlePreview = () => {
    if (suffix.value === 'preview') {
      previewRef.value.reloadPreview();
    } else {
      suffix.value = 'preview';
    }
  };
</script>

<style scoped lang="scss">
  .content {
    display: flex;
    height: 100%;
    &.fullscreen {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 100vh;
      z-index: 5000;
      flex: none;
    }
  }
  .config-content-editor {
    flex: 1;
    height: 100%;
    min-width: 0;
    .editor-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 16px;
      height: 40px;
      color: #979ba5;
      background: #2e2e2e;
      border-radius: 2px 2px 0 0;
      .highlight-select {
        width: 200px;
        :deep(.bk-input) {
          height: 26px;
          line-height: 26px;
          border: 1px solid #575757;
          input {
            background: #2e2e2e;
            color: #b3b3b3;
          }
        }
        .select-prefix {
          padding: 0 8px;
          background: #3d3d3d;
          color: #b3b3b3;
        }
      }
      .btns {
        display: flex;
        align-items: center;
        gap: 18px;
        & > span {
          cursor: pointer;
          &:hover {
            color: #3a84ff;
          }
        }
      }
    }
    .editor-content {
      height: calc(100% - 86px);
    }
    .editor-footer {
      display: flex;
      justify-content: flex-start;
      height: 46px;
      align-items: center;
      gap: 8px;
      padding: 12px 16px;
      background: #1a1a1a;
      .bk-button {
        width: 82px;
        height: 26px;
        border: 1px solid #575757;
        background: none;
        color: #b3b3b3;
      }
    }
  }
</style>
