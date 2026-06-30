<template>
  <bk-dialog
    ext-cls="confirm-dialog"
    :is-show="isShow"
    :show-mask="true"
    :quick-close="false"
    :multi-instance="false"
    @closed="emits('update:isShow', false)">
    <div class="title-icon"><Done fill="#42C06A" /></div>
    <div class="title-info">{{ isCreate ? $t('服务新建成功') : $t('服务克隆成功') }}</div>
    <div class="content-info">
      {{
        serviceData.config_type === 'file'
          ? $t('接下来你可以在服务下新增配置文件')
          : $t('接下来你可以在服务下新增配置项')
      }}
    </div>
    <div class="footer-btn">
      <bk-button theme="primary" @click="handleGoCreateConfig" style="margin-right: 8px">
        {{ serviceData.config_type === 'file' ? $t('新增配置文件') : $t('新增配置项') }}
      </bk-button>
      <bk-button @click="emits('update:isShow', false)">{{ $t('稍后再说') }}</bk-button>
    </div>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { IServiceEditForm } from '../../../../../../types/service';
  import { Done } from 'bkui-vue/lib/icon';
  import { useRouter } from 'vue-router';

  const router = useRouter();

  const props = withDefaults(
    defineProps<{
      bkBizId: string;
      appId: number;
      isShow: boolean;
      serviceData: IServiceEditForm;
      isCreate?: boolean;
    }>(),
    {
      isCreate: true,
    },
  );

  const emits = defineEmits(['update:isShow']);

  const handleGoCreateConfig = () => {
    emits('update:isShow', false);
    // 目前组件库dialog关闭自带250ms的延迟，所以这里延时300ms
    setTimeout(() => {
      router.push({
        name: 'service-config',
        params: {
          spaceId: props.bkBizId,
          appId: props.appId,
        },
      });
    }, 300);
  };
</script>

<style scoped lang="scss">
  :deep(.confirm-dialog) {
    .bk-modal-body {
      width: 400px;
      padding: 0;
      .bk-modal-header {
        display: none;
      }
      .bk-modal-footer {
        display: none;
      }
      .bk-modal-content {
        display: flex;
        flex-direction: column;
        align-items: center;
        .title-icon {
          margin: 27px 0 19px;
          width: 42px;
          height: 42px;
          border-radius: 50%;
          font-size: 42px;
          line-height: 42px;
          background-color: #e5f6e8;
        }
        .title-info {
          height: 32px;
          font-size: 20px;
          color: #313238;
          text-align: center;
          line-height: 32px;
        }
        .content-info {
          margin-top: 8px;
          height: 22px;
          font-size: 14px;
          color: #63656e;
          line-height: 22px;
        }
        .footer-btn {
          margin: 24px 0;
        }
      }
    }
  }
</style>
