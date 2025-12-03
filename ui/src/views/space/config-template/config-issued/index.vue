<template>
  <DetailLayout :name="$t('配置下发')" :show-footer="false">
    <template #header-suffix>
      <div class="steps-wrap">
        <bk-steps class="steps" theme="primary" :cur-step="stepsStatus.curStep" :steps="stepsStatus.steps" />
      </div>
    </template>
    <template #content>
      <div class="content">
        <SelectRange v-if="stepsStatus.curStep === 1" />
        <div class="op-btns">
          <bk-button v-if="stepsStatus.curStep === 1" theme="primary" @click="stepsStatus.curStep = 2">
            {{ t('下一步') }}
          </bk-button>
          <template v-else>
            <bk-button @click="stepsStatus.curStep = 1">{{ t('上一步') }}</bk-button>
            <bk-button theme="primary">{{ t('立即下发') }}</bk-button>
          </template>
          <bk-button @click="handleClose">{{ t('取消') }}</bk-button>
        </div>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import SelectRange from './select-range.vue';

  const { t } = useI18n();

  const emits = defineEmits(['close']);

  const stepsStatus = ref({
    steps: [{ title: t('选择范围') }, { title: t('配置生成') }],
    curStep: 1,
    controllable: true,
  });

  const handleClose = () => {
    emits('close');
  };
</script>

<style scoped lang="scss">
  .steps-wrap {
    flex: 1;
    .steps {
      width: 400px;
      margin: 0 auto;
    }
  }
  .content {
    padding: 24px;
    background: #f5f7fa;
    height: 100%;
  }
  .op-btns {
    display: flex;
    justify-content: center;
    gap: 8px;
    .bk-button {
      width: 88px;
    }
  }
</style>
