<template>
  <bk-dialog :is-show="isShow" :title="$t('更新托管信息')" width="960">
    <div class="info-wrap">
      <div class="info-content">
        <div v-for="value in 2" :key="value" class="info">
          <div class="info-title">
            <bk-tag :theme="value === 1 ? 'info' : 'success'">{{ value === 1 ? t('旧') : t('新') }}</bk-tag>
            <span class="title">进程别名</span>
          </div>
          <div class="content">
            <div v-for="info in value === 1 ? oldDisplayData : newDisplayData" :key="info.title" class="info-item">
              <div class="label">{{ info.title }}</div>
              <div :class="['value', { update: value === 2 && info.isWarn }]">
                {{ info.content }}
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="info-bottom">
        <div class="icon"></div>
        <span>{{ t('更新') }}</span>
      </div>
    </div>
    <bk-checkbox class="restart-checkbox" v-model="restart"> {{ t('重启') }}</bk-checkbox>
    <bk-alert v-show="restart" class="restart-alert" theme="warning">
      <template #title>
        <span>{{ $t('全部进程将会进行重启') }}:</span>
        <br />
        <span>{{ $t('执行旧的停止命令，使用新的启动命令') }}</span>
      </template>
    </bk-alert>
    <template #footer>
      <div class="button-group">
        <bk-button theme="primary" @click="handleSubmitClick">
          {{ restart ? t('更新并重启') : t('更新') }}
        </bk-button>
        <bk-button @click="handleClose">{{ t('取消') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();
  const props = defineProps<{
    isShow: boolean;
    managedInfo: {
      old: string;
      new: string;
    };
  }>();
  const emits = defineEmits(['close', 'update']);
  const newDisplayData = ref();
  const oldDisplayData = ref();
  const restart = ref(false);

  watch(
    () => props.managedInfo,
    () => {
      compareData(
        props.managedInfo.new && JSON.parse(props.managedInfo.new),
        props.managedInfo.old && JSON.parse(props.managedInfo.old),
      );
    },
    { deep: true },
  );

  // 转换成展示格式的函数
  const transformToDisplayFormat = (data: any) => {
    return [
      { title: t('进程启动参数：'), content: data.bk_start_param_regex || '--', isWarn: false },
      { title: t('工作路径：'), content: data.work_path || '--', isWarn: false },
      { title: t('PID 路径：'), content: data.pid_file || '--', isWarn: false },
      { title: t('启动用户：'), content: data.user || '--', isWarn: false },
      { title: t('启动命令：'), content: data.start_cmd || '--', isWarn: false },
      { title: t('停止命令：'), content: data.stop_cmd || '--', isWarn: false },
      { title: t('强制停止：'), content: data.face_stop_cmd || '--', isWarn: false },
      { title: t('重载命令：'), content: data.reload_cmd || '--', isWarn: false },
      { title: t('启动等待时长：'), content: `${data.start_check_secs}s` || '--', isWarn: false },
      { title: t('操作超时时长：'), content: `${data.timeout}s` || '--', isWarn: false },
    ];
  };

  const compareData = (newData: any, oldData: any) => {
    newDisplayData.value = transformToDisplayFormat(newData);
    oldDisplayData.value = transformToDisplayFormat(oldData);

    newDisplayData.value.forEach((newItem: any, index: number) => {
      const oldItem = oldDisplayData.value[index];
      if (newItem.content !== oldItem.content) {
        newItem.isWarn = true;
      }
    });

    return newDisplayData;
  };

  const handleSubmitClick = () => {
    emits('update', restart.value);
    emits('close');
  };
  const handleClose = () => {
    emits('close');
  };
</script>

<style scoped lang="scss">
  .info-wrap {
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
        .label {
          width: 110px;
          text-align: right;
          color: #4d4f56;
        }
        .value {
          width: 300px;
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
  .restart-checkbox {
    margin: 16px 0;
  }
  .restart-alert {
    margin-bottom: 24px;
  }
  .button-group {
    .bk-button {
      margin-left: 7px;
    }
  }
</style>
