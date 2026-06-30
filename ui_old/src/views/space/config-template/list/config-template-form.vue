<template>
  <div class="content">
    <div class="form-wrap">
      <div class="title">{{ $t('模板信息') }}</div>
      <div class="attribution">
        <span class="label">{{ $t('模板归属') }}</span>
        <span class="value">{{ attribution }}</span>
      </div>
      <bk-form ref="formRef" form-type="vertical" :model="localVal" :rules="rules">
        <bk-form-item :label="$t('模板名称')" property="template_name" required>
          <bk-input v-model="localVal.template_name" @change="change" />
        </bk-form-item>
        <bk-form-item :label="$t('配置文件名')" property="full_path" required>
          <bk-input v-model="localVal.full_path" @change="change" />
        </bk-form-item>
        <bk-form-item :label="$t('配置文件描述')" property="template_memo">
          <bk-input
            v-model="localVal.template_memo"
            type="textarea"
            :rows="3"
            :maxlength="200"
            @change="change"></bk-input>
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
        <bk-form-item :label="$t('目标平台')" property="file_mode" required>
          <bk-select v-model="localVal.file_mode" :clearable="false" :filterable="false" @change="change">
            <bk-option v-for="item in fileModes" :key="item.value" :id="item.value" :name="item.label" />
          </bk-select>
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
  const localVal = ref({ ...props.localVal });
  const content = ref(props.content);
  const fileModes = [
    { label: 'Unix and macOS', value: 'unix' },
    { label: 'Windows', value: 'win' },
  ];
  const rules = {
    template_memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
  };

  const change = () => {
    emits('change', localVal.value, content.value);
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
      overflow: auto;
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
