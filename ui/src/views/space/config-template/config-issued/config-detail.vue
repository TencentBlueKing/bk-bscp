<template>
  <bk-sideslider :is-show="isShow" width="960" quick-close @closed="emits('update:isShow', false)">
    <template #header>
      <div class="header">
        <span class="title">{{ $t('配置文件详情') }}</span>
        <span class="line"></span>
        <span class="file">{{ templateDetail.config_file_name }}</span>
      </div>
    </template>
    <div class="detail-content-area">
      <div class="detail-info">
        <div class="info-item" v-for="item in infoList" :key="item.label">
          <span class="label">{{ item.label }}</span>
          <span class="value">{{ templateDetail[item.value as keyof typeof templateDetail] }}</span>
        </div>
      </div>
      <div class="content">
        <CodeEditor :model-value="templateDetail.content" :editable="false" />
      </div>
    </div>
  </bk-sideslider>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { getGenerateResult } from '../../../../api/config-template';
  import CodeEditor from '../../../../components/code-editor/index.vue';

  const { t } = useI18n();

  const props = defineProps<{
    bkBizId: string;
    isShow: boolean;
    teskId: string;
  }>();
  const emits = defineEmits(['update:isShow']);
  const templateDetail = ref({
    config_file_group: '',
    config_file_name: '',
    config_file_owner: '',
    config_file_path: '',
    config_file_permission: '',
    config_instance_key: '',
    config_template_id: 0,
    config_template_name: '',
    content: '',
  });

  const infoList = [
    {
      label: t('模板名称'),
      value: 'config_template_name',
    },
    {
      label: t('文件名称'),
      value: 'config_file_name',
    },
    {
      label: t('文件路径'),
      value: 'config_file_path',
    },
    {
      label: t('文件权限'),
      value: 'config_file_permission',
    },
    {
      label: t('用户'),
      value: 'config_file_owner',
    },
    {
      label: t('用户组'),
      value: 'config_file_group',
    },
  ];

  watch(
    () => props.isShow,
    (newVal) => {
      if (newVal) {
        loadGenerateResult();
      }
    },
  );

  const loadGenerateResult = async () => {
    try {
      const res = await getGenerateResult(props.bkBizId, props.teskId);
      templateDetail.value = res;
    } catch (error) {
      console.error(error);
    }
  };
</script>

<style scoped lang="scss">
  .header {
    display: flex;
    align-items: center;
    .title {
      font-size: 16px;
      color: #313238;
    }
    .line {
      width: 1px;
      height: 16px;
      background: #c4c6cc;
      margin: 0 12px;
    }
    .file {
      font-size: 12px;
      color: #666666;
    }
  }

  .detail-content-area {
    display: flex;
    height: calc(100vh - 52px);
    padding: 20px 24px 24px 40px;
    .detail-info {
      flex: 1;
      width: 368px;
      .title {
        font-weight: 700;
        font-size: 14px;
        color: #4d4f56;
        line-height: 22px;
        margin-bottom: 16px;
      }
      .info-item {
        display: flex;
        flex-direction: column;
        font-size: 12px;
        line-height: 20px;
        margin-bottom: 24px;
        .label {
          color: #979ba5;
          margin-bottom: 4px;
        }
        .value {
          color: #313238;
        }
      }
    }
    .content {
      width: 716px;
    }
  }
</style>
