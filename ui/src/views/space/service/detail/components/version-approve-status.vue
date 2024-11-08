<template>
  <bk-loading
    v-if="approverList && route.params.versionId && showStatusIdArr.includes(Number(route.params.versionId))"
    :loading="loading"
    size="mini">
    <div class="version-approve-status">
      <Spinner v-show="approveStatus === 0" class="spinner" />
      <div
        v-show="approveStatus !== 0"
        :class="['dot', { online: approveStatus === 1, offline: [2, 3].includes(approveStatus) }]"></div>
      <span class="approve-status-text">{{ approveText }}</span>
      <text-file
        v-show="approveStatus > -1"
        v-bk-tooltips="{
          content: `${approveStatus === 3 ? t('撤销人') : t('审批人')}：${approverList}`,
          placement: 'bottom',
        }"
        class="text-file" />
    </div>
  </bk-loading>
</template>

<script setup lang="ts">
  import { ref, onMounted, watch } from 'vue';
  import { Spinner, TextFile } from 'bkui-vue/lib/icon';
  import { useRoute } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { versionStatusQuery } from '../../../../../api/config';
  import { APPROVE_TYPE } from '../../../../../constants/config';
  import { debounce } from 'lodash';

  const emits = defineEmits(['sendData']);

  const props = defineProps<{
    showStatusId: number; // 操作 提交上线/调整分组上线/撤销上线时的id
  }>();

  const route = useRoute();
  const { t } = useI18n();

  const approverList = ref(''); // 审批人
  const approveStatus = ref(-1); // 审批图标状态展示 0待审批 1上线 2驳回 3撤销
  const approveText = ref(''); // 审批文案
  const showStatusIdArr = ref<number[]>([]); // 提交上线/调整分组上线/撤销上线的id集合
  const loading = ref(true);

  watch(
    () => route.params.versionId,
    (newV) => {
      if (newV !== 'undefined') {
        loadStatus();
      }
    },
  );

  onMounted(async () => {
    await loadStatus();
    if ([0, 1].includes(approveStatus.value)) {
      showStatusIdArr.value.push(Number(route.params.versionId));
    }
  });

  const loadStatus = debounce(async () => {
    loading.value = true;
    if (route.params.versionId) {
      const { spaceId, appId, versionId } = route.params;
      try {
        const resp = await versionStatusQuery(String(spaceId), Number(appId), Number(versionId));
        const { spec } = resp.data;
        approverList.value = spec.approver_progress; // 审批人
        approveText.value = publishStatusText(spec.publish_status);
        sendData(resp.data);
        filterShowVer();
      } catch (error) {
        console.log(error);
      } finally {
        loading.value = false;
      }
    }
  }, 300);

  const publishStatusText = (type: string) => {
    switch (type as string) {
      case 'pending_approval':
        approveStatus.value = APPROVE_TYPE.pending_approval;
        return t('待审批');
      case 'pending_publish':
        approveStatus.value = APPROVE_TYPE.pending_publish;
        return t('审批通过');
      case 'rejected_approval':
        approveStatus.value = APPROVE_TYPE.rejected_approval;
        return t('审批驳回');
      case 'revoked_publish':
        approveStatus.value = APPROVE_TYPE.revoked_publish;
        return t('撤销上线');
      case 'already_publish':
      default:
        approveStatus.value = -1;
        return '';
    }
  };

  // 版本状态是否显示()
  const filterShowVer = () => {
    // 更新了版本状态的id都需要展示
    if (props.showStatusId > -1 && !showStatusIdArr.value.includes(props.showStatusId)) {
      showStatusIdArr.value.push(props.showStatusId);
    }
    // 如果当前版本状态为待审批/审批通过，也需要展示
    if ([0, 1].includes(approveStatus.value) && !showStatusIdArr.value.includes(Number(route.params.versionId))) {
      showStatusIdArr.value.push(Number(route.params.versionId));
    }
  };

  const sendData = (data: any) => {
    const { spec, revision } = data;
    const approveData = {
      status: spec.publish_status,
      time: spec.publish_time,
      type: spec.publish_type,
    };
    emits('sendData', approveData, revision?.creator || '');
  };

  defineExpose({
    loadStatus,
  });
</script>

<style lang="scss" scoped>
  .version-approve-status {
    margin: 0 16px 0 8px;
    display: flex;
    justify-content: center;
    align-items: center;
    .spinner {
      margin-right: 8px;
    }
    .text-file {
      font-size: 14px;
      color: #63656e;
    }
  }
  .approve-status-text {
    margin-right: 8px;
    font-size: 12px;
    color: #63656e;
  }
  .dot {
    margin-right: 8px;
    width: 13px;
    height: 13px;
    border-radius: 50%;
    &.online {
      border: 3px solid #e0f5e7;
      background-color: #3fc06d;
    }
    &.offline {
      border: 3px solid #eeeef0;
      background-color: #979ba5;
    }
  }
</style>
