<template>
  <bk-sideslider :is-show="props.show" :width="1200" @closed="handleClose">
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
              <div class="version-tag current-version">{{ t('实时') }}</div>
              <span class="timer">{{ $t('更新时间') }}: {{ configDiffData.current.createTime }}</span>
            </div>
          </slot>
        </template>
        <template #rightHead>
          <slot name="currentHead">
            <div class="diff-panel-head">
              <div class="version-tag base-version">{{ t('预生成') }}</div>
              <span class="timer">{{ $t('更新时间') }}: {{ configDiffData.base.createTime }}</span>
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
  import { compareConfigInstance } from '../../../../api/config-template';

  import Diff from '../../../../components/diff/index.vue';

  const { t } = useI18n();
  const props = defineProps<{
    show: boolean;
    spaceId: string;
    filePath: string;
    instance: {
      ccProcessId: number;
      moduleInstSeq: number;
      configVersionId: number;
    };
  }>();

  const emits = defineEmits(['update:show']);

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
  watch(
    () => props.show,
    (newVal) => {
      if (newVal) {
        loadGenerateResult();
      }
    },
  );

  const loadGenerateResult = async () => {
    try {
      const res = await compareConfigInstance(props.spaceId, props.instance);
      configDiffData.value = {
        contentType: 'text',
        id: 0,
        current: {
          content: res.oldConfigContent.content,
          createTime: res.oldConfigContent.createTime,
        },
        base: {
          content: res.newConfigContent.content,
          createTime: res.newConfigContent.createTime,
        },
      };
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
    emits('update:show', false);
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
      font-size: 12px;
      color: #666666;
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
