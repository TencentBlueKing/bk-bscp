<template>
  <div class="content">
    <div class="form-wrap">
      <div class="title">{{ $t('模板信息') }}</div>
      <div class="attribution">
        <span class="label">{{ $t('模板归属') }}</span>
        <span class="value">{{ attribution }}</span>
      </div>
      <bk-form ref="formRef" form-type="vertical" :model="localVal" :rules="rules">
        <bk-form-item :label="$t('模板名称')" property="name" required>
          <bk-input v-model="localVal.name" @change="change" />
        </bk-form-item>
        <bk-form-item :label="$t('配置文件名')" property="fileAP" required>
          <bk-input
            v-model="localVal.fileAP"
            :placeholder="t('请输入配置文件的完整路径和文件名，例如：/etc/nginx/nginx.conf')"
            v-bk-tooltips="{ content: t('请输入配置文件的完整路径和文件名，例如：/etc/nginx/nginx.conf') }"
            @input="handleFileAPInput" />
        </bk-form-item>
        <bk-form-item :label="$t('配置文件描述')" property="memo">
          <bk-input v-model="localVal.memo" type="textarea" :rows="3" :maxlength="200" @change="change"></bk-input>
        </bk-form-item>
        <bk-form-item :label="$t('文件权限')" property="privilege" required>
          <PermissionInputPicker v-model="localVal.privilege!" class="permission-input" @change="change" />
        </bk-form-item>
        <bk-form-item :label="$t('用户')" property="user" required>
          <bk-input v-model="localVal.user" class="permission-input" @change="change" />
        </bk-form-item>
        <bk-form-item :label="$t('用户组')" property="user_group" required>
          <bk-input v-model="localVal.user_group" class="permission-input" @change="change" />
        </bk-form-item>
      </bk-form>
    </div>
    <div class="editor-wrap">
      <ConfigContent
        :bk-biz-id="bkBizId"
        :highlight-style="localVal.highlight_style"
        :content="content"
        @highlight-change="handleHighlightChange"
        @change="handleContentChange" />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import PermissionInputPicker from '../../../../components/permission-input-picker.vue';
  import ConfigContent from '../components/config-content.vue';
  import SHA256 from 'crypto-js/sha256';
  import type { IConfigTemplateEditParams } from '../../../../../types/config-template';

  const { t } = useI18n();

  const props = defineProps<{
    attribution: string;
    localVal: IConfigTemplateEditParams;
    content: string;
    bkBizId: string;
    edit: boolean;
  }>();
  const emits = defineEmits(['change']);

  const formRef = ref();
  const localVal = ref({ ...props.localVal, fileAP: '' });
  const content = ref(props.content);
  const rules = {
    fileAP: [
      {
        validator: (val: string) => /^\/(?:[^/]+\/)*[^/]+$/.test(val),
        message: t('无效的路径,路径不符合Unix文件路径格式规范'),
        trigger: 'change',
      },
    ],
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
  };

  const change = () => {
    const { fileAP } = localVal.value;
    const lastSlashIndex = fileAP.lastIndexOf('/');
    localVal.value.file_name = fileAP.slice(lastSlashIndex + 1);
    localVal.value.file_path = fileAP.slice(0, lastSlashIndex + 1);
    emits('change', localVal.value, content.value);
  };

  const handleFileAPInput = () => {
    // 用户输入文件名 补全路径
    if (localVal.value.fileAP && !localVal.value.fileAP.startsWith('/')) {
      localVal.value.fileAP = `/${localVal.value.fileAP}`;
    }
    change();
  };

  const handleContentChange = (value: string) => {
    content.value = value;
    change();
  };

  const handleHighlightChange = (value: string) => {
    localVal.value.highlight_style = value;
    change();
  };

  defineExpose({
    getSignature: () => {
      if (!content.value.endsWith('\n')) content.value += '\n';
      return SHA256(content.value).toString();
    },
    validate: async () => {
      await formRef.value.validate();
      return true;
    },
  });
</script>

<style scoped lang="scss">
  .content {
    display: flex;
    height: calc(100% - 48px);
    background: #ffffff;
    .form-wrap {
      padding: 12px 24px;
      width: 368px;
      .title {
        font-weight: 700;
        font-size: 14px;
        color: #4d4f56;
        line-height: 22px;
        margin-bottom: 16px;
      }
      .attribution {
        display: flex;
        flex-direction: column;
        font-size: 12px;
        line-height: 20px;
        margin-bottom: 24px;
        .label {
          color: #4d4f56;
        }
        .value {
          color: #313238;
        }
      }
      .permission-input {
        width: 160px;
      }
    }
    .editor-wrap {
      flex: 1;
      min-width: 0;
    }
  }
</style>
