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
      <diff :diff="configDiffData" :is-tpl="true" :loading="loadng">
        <template #leftHead>
          <slot name="baseHead">
            <div class="diff-panel-head">
              <div class="version-tag current-version">{{ t('实时') }}</div>
              <span class="timer">{{ $t('更新时间') }}: {{ datetimeFormat(configDiffData.current.createTime!) }}</span>
            </div>
          </slot>
        </template>
        <template #rightHead>
          <slot name="currentHead">
            <div class="diff-panel-head">
              <div class="version-tag base-version">{{ t('预生成') }}</div>
              <span class="timer">{{ $t('更新时间') }}: {{ datetimeFormat(configDiffData.base.createTime!) }}</span>
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
  import { checkConfigView } from '../../../../api/config-template';

  import Diff from '../../../../components/diff/index.vue';
  import { datetimeFormat } from '../../../../utils';

  const { t } = useI18n();
  const props = defineProps<{
    show: boolean;
    spaceId: string;
    filePath: string;
    instance: {
      configTemplateId: number;
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
  const loadng = ref(false);

  watch(
    () => props.show,
    (newVal) => {
      if (newVal) {
        loadGenerateResult();
      }
    },
  );

  const loadGenerateResult = async () => {
    loadng.value = true;
    try {
      const params = {
        config_template_id: props.instance.configTemplateId,
        cc_process_id: props.instance.ccProcessId,
        module_inst_seq: props.instance.moduleInstSeq,
        config_version_id: props.instance.configVersionId,
      };
      const res = await checkConfigView(props.spaceId, params);
      configDiffData.value = {
        contentType: 'text',
        id: 0,
        current: {
          content: res.last_dispatched.data.content,
          createTime: res.last_dispatched.timestamp,
        },
        base: {
          content: res.preview_config.data.content,
          createTime: res.preview_config.timestamp,
        },
      };
    } catch (error) {
      console.error(error);
    } finally {
      loadng.value = false;
    }
  };

  const handleClose = () => {
    configDiffData.value = {
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
      font-size: 14px;
      color: #4d4f56;
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
