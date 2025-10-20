<template>
  <bk-dialog :is-show="isShow" :title="$t('更新托管信息')" width="960">
    <bk-alert theme="warning">
      <template #title>
        <span>{{ $t('全部进程将会进行重启：') }}</span>
        <br />
        <span>{{ $t('执行旧的停止命令，使用新的启动命令') }}</span>
      </template>
    </bk-alert>
    <div class="info-wrap">
      <div class="info-content">
        <div v-for="value in 2" :key="value" class="info">
          <div class="info-title">
            <bk-tag :theme="value === 1 ? 'info' : 'success'">{{ value === 1 ? t('旧') : t('新') }}</bk-tag>
            <span class="title">进程别名</span>
          </div>
          <div class="content">
            <div v-for="info in infoList" :key="info.label" class="info-item">
              <div class="label">{{ info.label }}</div>
              <span :class="{ update: value === 2 && info.warn }">{{ info.value }}</span>
            </div>
          </div>
        </div>
      </div>
      <div class="info-bottom">
        <div class="icon"></div>
        <span>{{ t('更新') }}</span>
      </div>
    </div>
    <template #footer>
      <div class="button-group">
        <bk-button theme="primary" @click="handleSubmitClick">
          {{ t('更新并重启') }}
        </bk-button>
        <bk-button @click="handleClose">{{ t('取消') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();
  defineProps<{
    isShow: boolean;
  }>();
  const emits = defineEmits(['close']);

  const infoList = [
    {
      label: t('进程启动参数：'),
      value: 'aa',
      warn: true,
    },
    {
      label: t('工作路径：'),
      value: 'bb',
    },
    {
      label: t('PID 路径：'),
      value: 'cc',
      warn: true,
    },
    {
      label: t('启动用户：'),
      value: 'dd',
    },
    {
      label: t('启动命令：'),
      value: 'ee',
    },
    {
      label: t('停止命令：'),
      value: 'ff',
    },
    {
      label: t('强制停止：'),
      value: 'gg',
    },
    {
      label: t('重载命令：'),
      value: 'hh',
    },
    {
      label: t('启动等待时长：'),
      value: 'ii',
    },
    {
      label: t('操作超时时长：'),
      value: 'jj',
    },
  ];

  const handleSubmitClick = () => {
    // TODO 提交更新托管信息
  };
  const handleClose = () => {
    emits('close');
  };
</script>

<style scoped lang="scss">
  .info-wrap {
    margin: 16px 0 24px;
    border: 1px solid #dcdee5;
    border-radius: 2px;
    .info-content {
      display: flex;
    }
    .info {
      width: 50%;
      color: #313238;
      &:first-child {
        border-right: 1px solid #dcdee5;
      }
    }
    .info-title {
      height: 42px;
      padding: 8px 16px;
      border-bottom: 1px solid #dcdee5;
      .title {
        margin-left: 8px;
      }
    }
    .content {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 14px 0;
      font-size: 12px;
      line-height: 20px;
      .info-item {
        display: flex;
        align-items: center;
        .label {
          width: 110px;
          text-align: right;
          color: #4d4f56;
        }
        .update {
          color: #e38b02;
        }
      }
    }
    .info-bottom {
      display: flex;
      align-items: center;
      gap: 8px;
      height: 32px;
      background: #f5f7fa;
      border-top: 1px solid #dcdee5;
      padding: 0 16px;
      .icon {
        width: 16px;
        height: 16px;
        background: #fdeed8;
        border: 1px solid #f59500;
        border-radius: 2px;
      }
    }
  }
  .button-group {
    .bk-button {
      margin-left: 7px;
    }
  }
</style>
