<template>
  <div class="version-editor-wrapper">
    <div :class="['version-editor', { 'open-suffix': suffix }]">
      <div class="header-wrapper">
        <span>{{ t('配置内容') }}</span>
        <div class="btns">
          <bk-select class="highlight-select" v-model="highlight" :filterable="false" :clearable="false">
            <template #prefix>
              <span class="select-prefix">{{ t('高亮风格') }}</span>
            </template>
            <bk-option v-for="(item, index) in highlightOptions" :id="item" :key="index" :name="item" />
          </bk-select>
        </div>
      </div>
      <div :class="['template-config-content-wrapper', { 'view-mode': isViewMode }]">
        <div v-if="!isViewMode" class="config-form">
          <bk-form ref="formRef" form-type="vertical" :model="formData" :rules="rules">
            <bk-form-item :label="t('模板名称')" property="name">
              <bk-input v-model="formData.name" />
            </bk-form-item>
            <bk-form-item :label="t('版本号')" property="revision_name">
              <bk-input v-model="formData.revision_name" />
            </bk-form-item>
            <bk-form-item :label="t('版本描述')" property="revision_memo">
              <bk-input
                v-model="formData.revision_memo"
                type="textarea"
                :placeholder="t('请输入')"
                :rows="4"
                :maxlength="200"
                :resize="true"
                :draggable="false" />
            </bk-form-item>
            <bk-form-item :label="t('文件权限')" required>
              <PermissionInputPicker v-model="formData.privilege" />
            </bk-form-item>
            <bk-form-item :label="t('用户')" required>
              <bk-input v-model="formData.user" />
            </bk-form-item>
            <bk-form-item :label="t('用户组')" required>
              <bk-input v-model="formData.user_group" />
            </bk-form-item>
          </bk-form>
        </div>
        <div v-bkloading="{ loading: contentLoading }" class="config-content">
          <CodeEditor v-model="stringContent" :language="highlight" :editable="!isViewMode" />
        </div>
      </div>
    </div>
    <ProcessPreview
      v-show="suffix === 'preview'"
      :bk-biz-id="spaceId"
      :config-content="stringContent"
      @close="suffix = ''" />
    <Variable v-show="suffix === 'variable'" :bk-biz-id="spaceId" @close="suffix = ''" />
  </div>
  <div class="action-btns">
    <bk-button v-if="isViewMode" theme="primary" @click="handleConfigIssue">
      {{ t('配置下发') }}
    </bk-button>
    <bk-button v-else theme="primary" @click="handleSubmitClick">{{ t('提交') }}</bk-button>
    <bk-button class="default-btn" @click="suffix = 'variable'">{{ t('变量') }}</bk-button>
    <bk-button class="default-btn" @click="suffix = 'preview'">{{ t('预览') }}</bk-button>
  </div>
</template>
<script lang="ts" setup>
  import { computed, onMounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import SHA256 from 'crypto-js/sha256';
  import Message from 'bkui-vue/lib/message';
  import { ITemplateVersionEditingData } from '../../../../../../types/template';
  import { stringLengthInBytes } from '../../../../../utils/index';
  import { updateTemplateContent, downloadTemplateContent } from '../../../../../api/template';
  import { createConfigTemplateVersion } from '../../../../../api/config-template';
  import { useRouter } from 'vue-router';
  import CodeEditor from '../../../../../components/code-editor/index.vue';
  import PermissionInputPicker from '../../../../../components/permission-input-picker.vue';
  import ProcessPreview from '../../components/process-preview.vue';
  import Variable from '../../components/variable.vue';

  const { t } = useI18n();
  const router = useRouter();
  const props = defineProps<{
    spaceId: string;
    templateSpaceId: number;
    templateId: number;
    configTemplateId: number;
    versionId: number;
    versionName: string;
    templateName: string;
    type: string;
    data: ITemplateVersionEditingData;
  }>();

  const emits = defineEmits(['created', 'close']);

  const rules = {
    revision_name: [
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
    revision_memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
  };

  const formData = ref<ITemplateVersionEditingData>({
    revision_name: '',
    revision_memo: '',
    file_type: '',
    file_mode: '',
    user: '',
    user_group: '',
    privilege: '',
    sign: '',
    byte_size: 0,
    name: props.templateName,
  });
  const formRef = ref();
  const stringContent = ref('');
  const contentLoading = ref(false);
  const uploadPending = ref(false);
  const submitPending = ref(false);
  const suffix = ref('');
  const highlight = ref('python');
  const highlightOptions = ['python', 'shell', 'bat', 'powershell'];

  const isViewMode = computed(() => props.type === 'view');

  watch(
    () => props.data,
    (val) => {
      formData.value = { ...val, name: props.templateName };
    },
    { immediate: true },
  );

  watch(
    () => props.versionId,
    (val) => {
      if (val) {
        getContent();
      }
    },
  );

  onMounted(() => {
    if (props.versionId) {
      getContent();
    }
  });

  // 获取非文件类型配置文件内容，文件类型手动点击时再下载
  const getContent = async () => {
    try {
      contentLoading.value = true;
      const { sign: signature } = props.data;
      const configContent = await downloadTemplateContent(props.spaceId, props.templateSpaceId, signature);
      stringContent.value = String(configContent);
    } catch (e) {
      console.error(e);
    } finally {
      contentLoading.value = false;
    }
  };

  // 上传配置内容
  const uploadContent = async () => {
    const signature = await getSignature();
    uploadPending.value = true;
    // @ts-ignore
    return updateTemplateContent(props.spaceId, props.templateSpaceId, stringContent.value, signature).then(() => {
      formData.value.byte_size = new Blob([stringContent.value]).size;
      formData.value.sign = signature;
      uploadPending.value = false;
    });
  };

  // 生成文件或文本的sha256
  const getSignature = async () => {
    return SHA256(stringContent.value).toString();
  };

  const validate = async () => {
    await formRef.value.validate();
    if (stringLengthInBytes(stringContent.value) > 1024 * 1024 * 100) {
      Message({ theme: 'error', message: `${t('配置内容不能超过')} 100M` });
      return false;
    }
    return true;
  };

  const handleSubmitClick = async () => {
    const result = await validate();
    if (!result) return;
    try {
      submitPending.value = true;
      await uploadContent();
      const res = await createConfigTemplateVersion(props.spaceId, props.configTemplateId, formData.value);
      emits('created', res.id);
      Message({
        theme: 'success',
        message: t('创建版本成功'),
      });
    } catch (e) {
      console.log(e);
    } finally {
      submitPending.value = false;
    }
  };

  // 配置下发
  const handleConfigIssue = () => {
    router.push({
      name: 'config-issued',
      query: {
        templateIds: [props.configTemplateId],
      },
    });
  };
</script>

<style lang="scss" scoped>
  .version-editor-wrapper {
    display: flex;
    height: calc(100% - 46px);
  }
  .version-editor {
    height: 100%;
    width: 100%;
    &.open-suffix {
      width: calc(100% - 417px);
    }
  }
  .header-wrapper {
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
  .title {
    display: flex;
    align-items: center;
    padding: 0 24px;
    height: 100%;
    line-height: 20px;
    font-size: 14px;
    color: #8a8f99;
  }
  .template-config-content-wrapper {
    display: flex;
    align-items: flex-start;
    height: calc(100% - 40px);
    &.view-mode {
      .config-content {
        width: 100%;
      }
    }
    .config-form {
      padding: 24px;
      width: 260px;
      height: 100%;
      background: #2a2a2a;
      overflow: auto;
      :deep(.bk-form) {
        .bk-form-label {
          font-size: 12px;
          color: #979ba5;
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
    .config-content {
      width: calc(100% - 260px);
      height: 100%;
    }
  }
  .permission-input-picker {
    :deep(.perm-panel-trigger) {
      background: #1e3250;
    }
  }
  .action-btns {
    padding: 7px 24px;
    background: #2a2a2a;
    box-shadow: 0 -1px 0 0 #141414;
    .bk-button {
      margin-right: 8px;
      min-width: 82px;
    }
    .default-btn {
      background: transparent;
      border-color: #979ba5;
      color: #979ba5;
    }
  }
</style>
