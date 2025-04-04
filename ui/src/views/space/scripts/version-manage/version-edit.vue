<template>
  <section :class="['script-content', { 'view-mode': !props.editable }, { 'show-variable': isShowVariable }]">
    <ScriptEditor
      v-model="localVal.content"
      v-model:is-show-variable="isShowVariable"
      :language="props.type"
      :editable="props.editable"
      :upload-icon="props.editable">
      <template #header>
        <div class="editor-header">
          <span class="title">{{ title }}</span>
          <span v-if="!editable" class="version-memo">{{ versionData.memo }}</span>
        </div>
      </template>
      <template v-if="props.editable" #preContent="{ fullscreen }">
        <div v-show="!fullscreen" class="version-config-form">
          <bk-form ref="formRef" :rules="rules" form-type="vertical" :model="localVal">
            <bk-form-item :label="t('版本号')" property="name">
              <bk-input v-model="localVal.name" :placeholder="t('请输入')" />
            </bk-form-item>
            <bk-form-item :label="t('版本说明')" propperty="memo">
              <bk-input
                v-model="localVal.memo"
                type="textarea"
                :placeholder="t('请输入')"
                :rows="8"
                :resize="true"
                :maxlength="200" />
            </bk-form-item>
          </bk-form>
        </div>
      </template>
      <template #sufContent>
        <InternalVariable v-show="isShowVariable" :language="props.type" />
      </template>
    </ScriptEditor>
    <div class="action-btns">
      <div v-if="props.editable">
        <bk-button class="submit-btn" theme="primary" :loading="pending" @click="handleSubmit">
          {{ t('保存') }}
        </bk-button>
        <bk-button class="cancel-btn" @click="emits('close')">{{ t('取消') }}</bk-button>
      </div>
      <div v-else>
        <bk-button
          v-if="['not_deployed', 'shutdown'].includes(hookRevision!.spec.state)"
          class="submit-btn"
          theme="primary"
          @click="emits('publish', hookRevision)">
          {{ t('上线') }}
        </bk-button>
        <bk-button v-if="hookRevision!.spec.state === 'not_deployed'" class="cancel-btn" @click="emits('edit')">
          {{ t('编辑') }}
        </bk-button>
        <bk-button
          v-if="hookRevision!.spec.state !== 'not_deployed'"
          class="submit-btn"
          theme="primary"
          :disabled="!!hasUnpublishVersion"
          v-bk-tooltips="{ content: t('当前已有「未上线」版本'), disabled: !hasUnpublishVersion }"
          @click="emits('copyAndCreate', hookRevision!.spec.content)">
          {{ t('复制并新建') }}
        </bk-button>
        <bk-button
          v-if="hookRevision!.spec.state === 'not_deployed'"
          class="cancel-btn"
          @click="emits('delete', hookRevision)">
          {{ t('删除') }}
        </bk-button>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
  import { ref, computed, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import BkMessage from 'bkui-vue/lib/message';
  import useGlobalStore from '../../../../store/global';
  import { IScriptVersion, IScriptVersionForm } from '../../../../../types/script';
  import { createScriptVersion, updateScriptVersion } from '../../../../api/script';
  import ScriptEditor from '../components/script-editor.vue';
  import InternalVariable from '../components/internal-variable.vue';

  const { spaceId } = storeToRefs(useGlobalStore());
  const { t } = useI18n();

  const props = withDefaults(
    defineProps<{
      type: string;
      editable?: boolean;
      scriptId: number;
      versionData: IScriptVersionForm;
      hookRevision: IScriptVersion | null;
      hasUnpublishVersion: boolean;
    }>(),
    {
      editable: true,
    },
  );

  const emits = defineEmits(['close', 'submitted', 'publish', 'edit', 'copyAndCreate', 'delete']);

  const rules = {
    name: [
      {
        validator: (value: string) => value.length <= 128,
        message: t('最大长度128个字符'),
      },
      {
        validator: (value: string) => {
          if (value.length > 0) {
            return /^[\u4e00-\u9fa5a-zA-Z0-9][\u4e00-\u9fa5a-zA-Z0-9_-]*[\u4e00-\u9fa5a-zA-Z0-9]?$/.test(value);
          }
          return true;
        },
        message: t('仅允许使用中文、英文、数字、下划线、中划线，且必须以中文、英文、数字开头和结尾'),
      },
    ],
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
  };
  const localVal = ref<IScriptVersionForm>({
    id: 0,
    name: '',
    memo: '',
    content: '',
  });
  const formRef = ref();
  const pending = ref(false);
  const isShowVariable = ref(true);

  const title = computed(() => {
    if (!props.editable) {
      return props.versionData.name;
    }
    return isEditVersion.value ? t('编辑版本') : t('新建版本');
  });

  const isEditVersion = computed(() => !!props.versionData.id);

  watch(
    () => props.versionData,
    (val) => {
      localVal.value = { ...val };
    },
    { immediate: true },
  );

  const handleSubmit = async () => {
    await formRef.value.validate();
    if (!localVal.value.content) {
      BkMessage({
        theme: 'error',
        message: t('脚本内容不能为空'),
      });
      return;
    }
    try {
      pending.value = true;
      if (!localVal.value.content.endsWith('\n')) {
        localVal.value.content += '\n';
      }
      const { name, memo, content } = localVal.value;
      const params = { name, memo, content };
      if (localVal.value.id) {
        await updateScriptVersion(spaceId.value, props.scriptId, localVal.value.id, params);
        emits('submitted', { ...localVal.value }, 'update');
        BkMessage({
          theme: 'success',
          message: t('编辑版本成功'),
        });
      } else {
        const res = await createScriptVersion(spaceId.value, props.scriptId, params);
        emits('submitted', { ...localVal.value, id: res.id }, 'create');
        BkMessage({
          theme: 'success',
          message: t('新建版本成功'),
        });
      }
    } catch (e) {
      console.error(e);
    } finally {
      pending.value = false;
    }
  };
</script>
<style lang="scss" scoped>
  .script-content {
    height: 100%;
    background: #2a2a2a;
    :deep(.script-editor) {
      height: calc(100% - 46px);
    }
    &.view-mode:not(.show-variable) {
      :deep(.script-editor) {
        .code-editor-wrapper {
          width: 100%;
        }
      }
    }
    &.show-variable:not(.view-mode) {
      :deep(.script-editor) {
        .code-editor-wrapper {
          width: calc(100% - 272px - 260px);
        }
      }
    }
    &.show-variable.view-mode {
      :deep(.script-editor) {
        .code-editor-wrapper {
          width: calc(100% - 272px);
        }
      }
    }
  }
  .editor-header {
    padding: 10px 24px;
    line-height: 20px;
    font-size: 14px;
    .title {
      color: #c3c5cb;
    }
    .version-memo {
      font-size: 12px;
      color: #63656e;
      margin-left: 16px;
    }
  }
  .version-config-form {
    padding: 24px 20px 24px;
    width: 260px;
    :deep(.bk-form) {
      .bk-form-label {
        font-size: 12px;
        color: #979ba5;
      }
      .bk-form-item {
        margin-bottom: 40px !important;
      }
      .bk-input {
        border: 1px solid #63656e;
      }
      .bk-input--text {
        background: transparent;
        color: #c4c6cc;
        &::placeholder {
          color: #63656e;
        }
      }
      .bk-textarea {
        background: transparent;
        border: 1px solid #63656e;
        textarea {
          color: #c4c6cc;
          background: transparent;
          &::placeholder {
            color: #63656e;
          }
        }
      }
    }
  }
  :deep(.script-editor) {
    .content-wrapper {
      display: flex;
      justify-content: space-between;
      height: calc(100% - 40px);
    }
    .code-editor-wrapper {
      width: calc(100% - 260px);
    }
  }
  .action-btns {
    padding: 7px 24px;
    background: #2a2a2a;
    box-shadow: 0 -1px 0 0 #141414;
    .submit-btn {
      margin-right: 8px;
      min-width: 120px;
    }
    .cancel-btn {
      min-width: 88px;
      background: transparent;
      border-color: #979ba5;
      color: #979ba5;
      margin-right: 8px;
    }
  }
</style>
