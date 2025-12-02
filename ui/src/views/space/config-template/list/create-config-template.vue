<template>
  <DetailLayout :name="$t('新建配置模板')" :show-footer="false" @close="handleClose">
    <template #content>
      <section class="content-wrap">
        <ConfigTemplateForm
          ref="formRef"
          :bk-biz-id="bkBizId"
          :attribution="attribution"
          :local-val="formData"
          :content="content"
          @change="handleFormChange" />
        <div class="btns">
          <bk-button theme="primary" @click="handleCreateConfirm">{{ $t('创建') }}</bk-button>
          <bk-button @click="handleClose">{{ $t('取消') }}</bk-button>
        </div>
      </section>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import ConfigTemplateForm from './config-template-form.vue';
  import { getConfigTemplateEditParams } from '../../../../utils/config-template';
  import { updateTemplateContent } from '../../../../api/template';
  import { createConfigTemplate } from '../../../../api/config-template';
  import type { IConfigTemplateEditParams } from '../../../../../types/config-template';
  import { Message } from 'bkui-vue';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const emits = defineEmits(['close', 'created']);
  const props = defineProps<{
    attribution: string;
    bkBizId: string;
    templateId: number;
  }>();

  const formData = ref<IConfigTemplateEditParams>(getConfigTemplateEditParams());
  const content = ref('');
  const formRef = ref();
  const pending = ref(false);

  const handleFormChange = (data: IConfigTemplateEditParams, formContent: string) => {
    formData.value = data;
    content.value = formContent;
  };

  const handleCreateConfirm = async () => {
    try {
      const isValid = await formRef.value.validate();
      if (!isValid) return;
      pending.value = true;
      const sign = await formRef.value.getSignature();
      const size = new Blob([content.value]).size;
      await updateTemplateContent(props.bkBizId, Number(props.bkBizId), content.value, sign);
      const params = {
        ...formData.value,
        ...{ sign, byte_size: size },
      };
      await createConfigTemplate(props.bkBizId, params);
      emits('created');
      Message({
        theme: 'success',
        message: t('新建配置文件成功'),
      });
      handleClose();
    } catch (e) {
      console.log(e);
    } finally {
      pending.value = false;
    }
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
