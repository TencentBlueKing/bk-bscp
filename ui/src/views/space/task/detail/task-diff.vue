<template>
  <bk-sideslider :is-show="props.isShow" :width="1200" @closed="handleClose">
    <template #header>
      <div class="header">
        <span class="title">{{ $t('配置对比') }}</span>
        <span class="line"></span>
        <span class="path">{{ filePath }}</span>
      </div>
    </template>
    <div class="diff-content-area">
      <diff :diff="configDiffData" :is-tpl="true" :loading="false">
        <template #leftHead>
          <slot name="baseHead">
            <div class="diff-panel-head">
              <div class="version-tag current-version">{{ t('最后下发') }}</div>
              <span class="timer">{{ $t('下发时间') }}: {{ datetimeFormat(configDiffData.current.createTime!) }}</span>
            </div>
          </slot>
        </template>
        <template #rightHead>
          <slot name="currentHead">
            <div class="diff-panel-head">
              <div class="version-tag base-version">{{ t('现网配置') }}</div>
              <span class="timer">{{ $t('检查时间') }}: {{ datetimeFormat(configDiffData.base.createTime!) }}</span>
            </div>
          </slot>
        </template>
      </diff>
    </div>
  </bk-sideslider>
</template>
<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { IDiffDetail } from '../../../../../types/service';
  import { taskCompare } from '../../../../api/task';
  import { datetimeFormat } from '../../../../utils';
  import Diff from '../../../../components/diff/index.vue';

  const { t } = useI18n();
  const props = defineProps<{
    isShow: boolean;
    bkBizId: string;
    taskId: string;
  }>();

  const emits = defineEmits(['update:isShow']);

  const configDiffData = ref<IDiffDetail>({
    contentType: 'text',
    id: 0,
    current: {
      content: '',
      createTime: '',
    },
    base: {
      content: '',
      createTime: '',
    },
  });
  const filePath = ref('');
  watch(
    () => props.isShow,
    (newVal) => {
      if (newVal) {
        loadDiff();
      }
    },
  );

  const loadDiff = async () => {
    try {
      const res = await taskCompare(props.bkBizId, props.taskId);
      configDiffData.value = {
        contentType: 'text',
        id: 0,
        current: {
          content: res.last_dispatched ? res.last_dispatched.data.content : '',
          createTime: res.last_dispatched ? res.last_dispatched.timestamp : '',
        },
        base: {
          content: res.preview_config ? res.preview_config.data.content : '',
          createTime: res.preview_config ? res.preview_config.timestamp : '',
        },
      };
      filePath.value = `${res.config_template_name} (${res.config_file_path})`;
    } catch (error) {
      console.error(error);
    }
  };

  const handleClose = () => {
    configDiffData.value = {
      contentType: 'text',
      id: 0,
      current: {
        content: '',
      },
      base: {
        content: '',
      },
    };
    emits('update:isShow', false);
  };
</script>
<style lang="scss" scoped>
  .diff-content-area {
    height: calc(100vh - 52px);
  }
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
    .path {
      font-size: 14px;
      color: #4D4F56;
    }
  }
  .diff-panel-head {
    display: flex;
    align-items: center;
    padding: 0 16px;
    width: 100%;
    height: 100%;
    font-size: 12px;
    color: #b6b6b6;
    background: #313238;
    .version-tag {
      margin-right: 8px;
      padding: 0 10px;
      height: 22px;
      line-height: 22px;
      font-size: 12px;
      border-radius: 2px;
      &.current-version {
        color: #64a0fa;
        background: #1e3567;
      }
      &.base-version {
        color: #3fc362;
        background: #144628;
      }
    }
    .timer {
      font-size: 12px;
      color: #b6b6b6;
    }
  }
</style>
