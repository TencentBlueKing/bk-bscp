<template>
  <DetailLayout :name="$t('配置模板详情')" :show-footer="false" @close="handleClose">
    <template #content>
      <section class="content-wrap">
        <div class="content">
          <div class="form-wrap">
            <div class="title">{{ $t('模板信息') }}</div>
            <div class="attribution">
              <span class="label">{{ $t('模板归属') }}</span>
              <span class="value">config_delivery/默认模板</span>
            </div>
            <bk-form ref="formRef" form-type="vertical" :model="formData" :rules="rules">
              <bk-form-item :label="$t('模板名称')" property="template_name" required>
                <bk-input v-model="formData.template_name"></bk-input>
              </bk-form-item>
              <bk-form-item :label="$t('配置文件名')" property="file_name" required>
                <bk-input v-model="formData.file_name"></bk-input>
              </bk-form-item>
              <bk-form-item :label="$t('配置文件描述')" property="memo">
                <bk-input v-model="formData.memo" type="textarea" :rows="3" :maxlength="200"></bk-input>
              </bk-form-item>
              <bk-form-item :label="$t('文件权限')" property="privilege" required>
                <PermissionInputPicker v-model="formData.privilege" class="permission-input" />
              </bk-form-item>
              <bk-form-item :label="$t('用户')" property="user" required>
                <bk-input v-model="formData.user" class="permission-input" />
              </bk-form-item>
              <bk-form-item :label="$t('用户组')" property="user_group" required>
                <bk-input v-model="formData.user_group" class="permission-input" />
              </bk-form-item>
            </bk-form>
          </div>
          <div class="editor-wrap">
            <ConfigContent :content="formData.content" />
          </div>
        </div>
        <div class="btns">
          <bk-button theme="primary" @click="handleConfirm">{{ $t('创建') }}</bk-button>
          <bk-button @click="handleClose">{{ $t('取消') }}</bk-button>
        </div>
      </section>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import PermissionInputPicker from '../../../../components/permission-input-picker.vue';
  import ConfigContent from '../components/config-content.vue';

  const { t } = useI18n();

  const emits = defineEmits(['close', 'created']);

  const formRef = ref();
  const formData = ref({
    privilege: '644',
    user: 'root',
    user_group: 'root',
    template_name: '',
    file_name: '',
    memo: '',
    content: '',
  });
  const rules = {
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
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
  };

  const handleConfirm = async () => {
    await formRef.value.validate();
  };

  const handleClose = () => {
    emits('close', false);
  };
</script>

<style scoped lang="scss">
  .content-wrap {
    padding: 24px;
    height: 100%;
    background: #f5f7fa;
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
        background: black;
        min-width: 0;
      }
    }
    .btns {
      margin-top: 16px;
      display: flex;
      gap: 16px;
      .bk-button {
        width: 88px;
      }
    }
  }
</style>
