<template>
  <Teleport :disabled="!isOpenFullScreen" to="body">
    <div :class="['config-content-editor', { fullscreen: isOpenFullScreen, 'show-example': modelValue }]">
      <div class="editor-title">
        <div class="tips">
          <div class="title">{{ t('导入数据') }}</div>
          <InfoLine class="info-icon" />
          {{ t('仅支持大小不超过') }}2M
        </div>
        <div class="btns">
          <i
            v-if="format === 'text'"
            :class="['bk-bscp-icon', 'icon-separator', { isOpen: separatorShow }]"
            v-bk-tooltips="{
              content: t('分隔符'),
              placement: 'top',
              distance: 20,
            }"
            @click="separatorShow = !separatorShow" />
          <Search
            v-bk-tooltips="{
              content: t('搜索'),
              placement: 'top',
              distance: 20,
            }"
            @click="codeEditorRef.openSearch()" />
          <bk-upload
            :accept="format === 'text' ? '.txt' : `.${format}`"
            theme="button"
            :size="5"
            :custom-request="handleUploadFile">
            <template #trigger>
              <Upload
                v-bk-tooltips="{
                  content: t('上传'),
                  placement: 'top',
                  distance: 20,
                }" />
            </template>
          </bk-upload>
          <i
            :class="['bk-bscp-icon', 'icon-terminal', { isOpen: modelValue }]"
            v-bk-tooltips="{
              content: t('示例面板'),
              placement: 'top',
              distance: 20,
            }"
            @click="emits('update:modelValue', !modelValue)" />
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
          :model-value="editorContent"
          :error-line="errorLine"
          :language="format"
          :file-editor="false"
          @enter="separatorShow = true"
          @paste="handlePaste"
          @update:model-value="handleContentChange" />
        <div v-if="format === 'text' && separatorShow" class="separator">
          <SeparatorSelect @closed="separatorShow = false" @confirm="handleSelectSeparator" />
        </div>
        <slot name="sufContent" :fullscreen="isOpenFullScreen"></slot>
      </div>
    </div>
  </Teleport>
</template>
<script setup lang="ts">
  import { ref, onBeforeUnmount, watch, computed, nextTick } from 'vue';
  import { useI18n } from 'vue-i18n';
  import BkMessage from 'bkui-vue/lib/message';
  import { InfoLine, FilliscreenLine, UnfullScreen, Search, Upload } from 'bkui-vue/lib/icon';
  import CodeEditor from '../../../../../../components/code-editor/index.vue';
  import SeparatorSelect from '../../../../variables/variable-import/separator-select.vue';
  import { IConfigKvItem } from '../../../../../../../types/config';
  import { importKvFormText, importKvFormJson, importKvFormYaml } from '../../../../../../api/config';
  import useServiceStore from '../../../../../../store/service';
  import yaml from 'js-yaml';

  interface errorLineItem {
    lineNumber: number;
    errorInfo: string;
  }

  const { t } = useI18n();
  const emits = defineEmits(['hasError', 'update:modelValue']);

  const serviceStore = useServiceStore();

  const isOpenFullScreen = ref(false);
  const codeEditorRef = ref();
  const separatorShow = ref(false);
  const textContent = ref('');
  const kvs = ref<IConfigKvItem[]>([]);
  const separator = ref(' ');
  const shouldValidate = ref(false);
  const errorLine = ref<errorLineItem[]>([]);
  const jsonContent = ref('');
  const yamlContent = ref('');

  const props = defineProps<{
    bkBizId: string;
    appId: number;
    modelValue: boolean;
    format: string;
  }>();

  const editorContent = computed(() => {
    if (props.format === 'text') return textContent.value;
    if (props.format === 'json') return jsonContent.value;
    return yamlContent.value;
  });

  watch(
    () => editorContent.value,
    (val) => {
      if (!val) {
        emits('hasError', true);
        return;
      }
      if (props.format === 'text') {
        handleValidateEditor();
      } else {
        if (props.format === 'json') {
          handleValidateJson();
        } else {
          handleValidateYaml();
        }
        nextTick(() => emits('hasError', errorLine.value.length || !codeEditorRef.value.validate(val)));
      }
    },
    { immediate: true },
  );

  watch(
    () => errorLine.value,
    (val) => {
      shouldValidate.value = val.length > 0;
    },
  );

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

  // 校验编辑器内容 处理上传kv格式
  const handleValidateEditor = () => {
    const kvArray = textContent.value.split('\n').map((item) => item.trim());
    errorLine.value = [];
    kvs.value = [];
    let hasSeparatorError = false;
    kvArray.forEach((item, index) => {
      if (item === '') return;
      const regex = separator.value === ' ' ? /\s+/ : separator.value;
      const kvContent = item.split(regex).map((item) => item.trim());
      const key = kvContent[0];
      const kv_type = kvContent[1] ? kvContent[1].toLowerCase() : '';
      let secret_type = '';
      let value = '';
      let secret_hidden = 'visible';
      let memo = '';
      if (kv_type === 'secret') {
        // 敏感信息批量导入
        secret_type = kvContent[2];
        value = kvContent[3];
        secret_hidden = kvContent[4];
        memo = kvContent[5] || '';
      } else {
        // 普通kv导入
        value = kvContent[2];
        memo = kvContent[3] || '';
      }
      if (kvContent.length < 3) {
        errorLine.value.push({
          errorInfo: t('请检查是否已正确使用分隔符'),
          lineNumber: index + 1,
        });
        hasSeparatorError = true;
      } else if (kv_type !== 'string' && kv_type !== 'number' && kv_type !== 'secret') {
        errorLine.value.push({
          errorInfo: t('类型必须为 string , number 或者secret'),
          lineNumber: index + 1,
        });
      } else if (kv_type === 'number' && !/^\d+(\.\d+)?$/.test(value)) {
        errorLine.value.push({
          errorInfo: t('类型为number 值不为number'),
          lineNumber: index + 1,
        });
      } else if (value === '') {
        errorLine.value.push({
          errorInfo: t('value不能为空'),
          lineNumber: index + 1,
        });
      } else if (kv_type === 'secret') {
        if (secret_type !== 'password' && secret_type !== 'secret_key' && secret_type !== 'token') {
          errorLine.value.push({
            errorInfo: t('敏感信息类型必须为password,secret_key,token'),
            lineNumber: index + 1,
          });
        }
        if (secret_hidden !== 'visible' && secret_hidden !== 'invisible') {
          errorLine.value.push({
            errorInfo: t('是否可见必须为visible或者invisible'),
            lineNumber: index + 1,
          });
        }
      }
      kvs.value.push({
        key,
        kv_type,
        secret_type,
        secret_hidden: secret_hidden === 'invisible',
        value,
        memo,
      });
    });
    emits('hasError', textContent.value && errorLine.value.length > 0);
    return hasSeparatorError;
  };

  const handleValidateJson = () => {
    const jsonObject = JSON.parse(jsonContent.value);
    const keys = Object.keys(jsonObject).filter((key) => jsonObject[key].value === '不可见敏感信息无法导出');
    const lines = jsonContent.value.split('\n');
    errorLine.value = [];
    lines.forEach((line, index) => {
      const match = line.match(/"value":\s*"(不可见敏感信息无法导出)"/);
      if (match) {
        errorLine.value.push({
          errorInfo: t('请先填写配置项 {n} 的值，然后再尝试导入', { n: keys[errorLine.value.length] }),
          lineNumber: index + 1,
        });
      }
    });
  };

  const handleValidateYaml = () => {
    try {
      const yamlObject = yaml.load(yamlContent.value);
      const allKeys = Object.keys(yamlObject);
      const secretKeys = allKeys.filter((key) => yamlObject[key].value === '不可见敏感信息无法导出');
      // 匹配所有不符合规则的key
      const errorKeys = allKeys.filter(
        (key) => !/^[\p{Script=Han}\p{L}\p{N}]([\p{Script=Han}\p{L}\p{N}_-]*[\p{Script=Han}\p{L}\p{N}])?$/u.test(key),
      );
      const lines = yamlContent.value.split('\n');
      errorLine.value = [];
      lines.forEach((line, index) => {
        const secretMatch = line.match(/value:\s*不可见敏感信息无法导出/);
        if (secretMatch) {
          errorLine.value.push({
            errorInfo: t('请先填写配置项 {n} 的值，然后再尝试导入', { n: secretKeys[errorLine.value.length] }),
            lineNumber: index + 1,
          });
        }
        // 如果该行包含不符合规则的键，记录下错误信息和行号
        errorKeys.forEach((key) => {
          if (line.includes(key)) {
            errorLine.value.push({
              errorInfo: `键 "${key}" 不符合规则`,
              lineNumber: index + 1,
            });
          }
        });
      });
    } catch (e) {
      console.error(e);
    }
  };

  // 导入kv
  const handleImport = async () => {
    let res;
    if (props.format === 'text') {
      res = await importKvFormText(props.bkBizId, props.appId, kvs.value, false);
    } else if (props.format === 'json') {
      res = await importKvFormJson(props.bkBizId, props.appId, jsonContent.value);
    } else {
      res = await importKvFormYaml(props.bkBizId, props.appId, yamlContent.value);
    }
    serviceStore.$patch((state) => {
      state.topIds = res.data.ids;
    });
  };

  const handleSelectSeparator = (selectSeparator: string) => {
    separator.value = selectSeparator;
    handleValidateEditor();
  };

  const handlePaste = () => {
    if (props.format === 'text' && handleValidateEditor()) {
      separatorShow.value = true;
    }
  };

  const handleUploadFile = async (option: { file: File }) => {
    const reader = new FileReader();
    reader.readAsText(option.file);
    reader.onload = function (e) {
      const fileContent = e.target?.result as string;
      if (props.format === 'text') {
        textContent.value = fileContent;
      } else if (props.format === 'json') {
        jsonContent.value = fileContent;
      } else {
        yamlContent.value = fileContent;
      }
    };
  };

  const handleContentChange = (val: string) => {
    if (props.format === 'text') {
      textContent.value = val;
    } else if (props.format === 'json') {
      jsonContent.value = val;
    } else {
      yamlContent.value = val;
    }
  };

  defineExpose({
    handleImport,
    handleValidate: () => {
      if (codeEditorRef.value && props.format === 'text') {
        return errorLine.value.length;
      }
      return codeEditorRef.value.validate(codeEditorRef.value);
    },
  });
</script>
<style lang="scss" scoped>
  .config-content-editor {
    height: 640px;
    &.fullscreen {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 100vh;
      z-index: 5000;
      &.show-example {
        :deep(.code-editor-wrapper) {
          width: calc(100vw - 520px);
        }
        :deep(.example-wrap) {
          position: absolute;
          right: 0;
          top: 0;
          height: 100%;
          .content {
            height: calc(100% - 40px);
            .example {
              height: 100%;
            }
            .bk-textarea {
              height: 100%;
            }
          }
        }
      }
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
      box-shadow: 0 2px 4px 0 #00000029;
      .tips {
        display: flex;
        align-items: center;
        font-size: 12px;
        .title {
          font-size: 14px;
          color: #c4c6cc;
          margin-right: 16px;
        }
        .info-icon {
          margin-right: 4px;
          font-size: 14px;
        }
      }
      .btns {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 8px;
        span,
        i {
          display: flex;
          align-items: center;
          justify-content: center;
          width: 24px;
          height: 24px;
          border-radius: 2px;
          font-size: 16px;
          color: #979ba5;
          cursor: pointer;
          &:hover {
            color: #3a84ff;
          }
          &.isOpen {
            background: #181818;
          }
        }
        :deep(.bk-upload) {
          display: flex;
          justify-content: space-between;
          align-items: center;
          .bk-upload-list {
            display: none;
          }
        }
      }
    }
    .editor-content {
      position: relative;
      height: calc(100% - 40px);
      .separator {
        position: absolute;
        right: 10px;
        top: -13px;
      }
    }
  }
</style>
