<template>
  <Teleport :disabled="!isOpenFullScreen" to="body">
    <div :class="['config-content-editor', { fullscreen: isOpenFullScreen }]" :style="editorStyle">
      <div class="editor-title">
        <div class="tips">
          <InfoLine class="info-icon" />
          {{ t('仅支持大小不超过') }}1M
        </div>
        <div class="btns">
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
          :language="languages"
          :always-consume-mouse-wheel="false"
          :file-editor="false"
          @update:model-value="emits('change', $event)" />
      </div>
    </div>
  </Teleport>
</template>
<script setup lang="ts">
  import { ref, computed, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import BkMessage from 'bkui-vue/lib/message';
  import { InfoLine, FilliscreenLine, UnfullScreen } from 'bkui-vue/lib/icon';
  import CodeEditor from '../../../../../../components/code-editor/index.vue';

  const { t } = useI18n();
  const props = withDefaults(
    defineProps<{
      content: string;
      languages: string;
      editable?: boolean;
      height?: number;
    }>(),
    {
      editable: true,
      height: 640,
    },
  );

  const emits = defineEmits(['change']);

  const isOpenFullScreen = ref(false);
  const codeEditorRef = ref();

  const editorStyle = computed(() => {
    return {
      height: isOpenFullScreen.value ? '100vh' : `${props.height}px`,
    };
  });

  onBeforeUnmount(() => {
    codeEditorRef.value.destroy();
  });

  // 打开全屏
  const handleOpenFullScreen = () => {
    isOpenFullScreen.value = true;
    window.addEventListener('keydown', handleEscClose, { once: true });
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
      isOpenFullScreen.value = false;
    }
  };

  defineExpose({
    validate: () => {
      if (codeEditorRef.value) {
        return codeEditorRef.value.validateAndMarkErrorLine(props.content);
      }
    },
  });
</script>
<style lang="scss" scoped>
  .config-content-editor {
    &.fullscreen {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      z-index: 5000;
    }
    .editor-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 16px;
      height: 40px;
      color: #979ba5;
      background: #2e2e2e;
      border-radius: 2px 2px 0 0;
      .tips {
        display: flex;
        align-items: center;
        font-size: 12px;
        .info-icon {
          margin-right: 4px;
        }
      }
      .btns {
        display: flex;
        align-items: center;
        & > span {
          cursor: pointer;
          &:hover {
            color: #3a84ff;
          }
        }
      }
    }
    .editor-content {
      height: calc(100% - 40px);
    }
  }
</style>
