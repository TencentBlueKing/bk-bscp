<template>
  <DetailLayout :name="$t('配置下发')" @close="handleClose">
    <template #header-suffix>
      <div class="steps-wrap">
        <bk-steps class="steps" theme="primary" :cur-step="stepsStatus.curStep" :steps="stepsStatus.steps" />
      </div>
    </template>
    <template #content>
      <div class="content">
        <SelectRange v-if="stepsStatus.curStep === 1" :bk-biz-id="spaceId" />
      </div>
    </template>
    <template #footer>
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
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { useRouter, useRoute } from 'vue-router';
  import DetailLayout from '../../scripts/components/detail-layout.vue';
  import SelectRange from './select-range/index.vue';
  import useGlobalStore from '../../../../store/global';

  const { t } = useI18n();
  const { spaceId } = storeToRefs(useGlobalStore());
  const router = useRouter();
  const route = useRoute();

  const stepsStatus = ref({
    steps: [{ title: t('选择范围') }, { title: t('配置生成') }],
    curStep: 1,
    controllable: true,
  });

  const handleClose = () => {
    if (route.query.processIds) {
      // 进程管理跳转配置下发
      router.push({
        name: 'process-management',
      });
    } else {
      router.push({
        name: 'config-template-list',
      });
    }
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
