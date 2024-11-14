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
      <bk-popover :popover-delay="[0, 300]" placement="bottom-end" theme="light">
        <text-file v-show="approveStatus > -1 && approveStatus !== 1" class="text-file" />
        <template #content>
          <div class="popover-content">
            <template v-if="itsmData?.itsm_ticket_sn">
              <div class="itsm-title">{{ $t('审批单') }}：</div>
              <div class="itsm-content em">
                <div class="itsm-sn" @click="handleLinkTo(itsmData?.itsm_ticket_url)">
                  {{ itsmData?.itsm_ticket_sn }}
                </div>
                <div class="itsm-action" @click="handleCopy(itsmData?.itsm_ticket_url)"><Copy /></div>
              </div>
            </template>
            <div class="itsm-title">
              {{
                `${approveStatus === 3 ? t('撤销人') : t('审批人')}（${approveType === 'or_sign' ? $t('或签') : $t('会签')}）`
              }}：
            </div>
            <div class="itsm-content">{{ approverList }}</div>
            <template v-if="approveStatus === 0 && publishTime">
              <div class="itsm-title">{{ $t('定时上线') }}：</div>
              <div class="itsm-content">
                {{ convertTime(publishTime, 'local') || '--' }}
              </div>
            </template>
            <template v-if="approveStatus === 2">
              <div class="itsm-title">{{ $t('驳回原因') }}：</div>
              <div class="itsm-content">
                {{ rejectionReason || '--' }}
              </div>
            </template>
          </div>
        </template>
      </bk-popover>
    </div>
  </bk-loading>
</template>

<script setup lang="ts">
  import { ref, onMounted, watch, onUnmounted } from 'vue';
  import { Spinner, TextFile, Copy } from 'bkui-vue/lib/icon';
  import { useRoute } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { versionStatusQuery } from '../../../../../api/config';
  import { APPROVE_TYPE } from '../../../../../constants/config';
  import { debounce } from 'lodash';
  import { convertTime, copyToClipBoard } from '../../../../../utils/index';
  import BkMessage from 'bkui-vue/lib/message';
  import { storeToRefs } from 'pinia';
  import useConfigStore from '../../../../../store/config';

  const emits = defineEmits(['send-data']);

  const props = defineProps<{
    showStatusId: number; // 操作 提交上线/调整分组上线/撤销上线时的id
    refreshVer: Function; // 刷新左侧版本列表
  }>();

  const route = useRoute();
  const { t } = useI18n();
  const versionStore = useConfigStore();
  const { versionData, publishedVersionId } = storeToRefs(versionStore);

  const approverList = ref(''); // 审批人
  const approveStatus = ref(-1); // 审批图标状态展示 0待审批 1待上线(审批通过) 2驳回 3撤销
  const approveText = ref(''); // 审批文案
  const approveType = ref(''); // 审批方式
  const rejectionReason = ref(''); // 拒绝理由
  const publishTime = ref(''); // 定时上线时间
  const showStatusIdArr = ref<number[]>([]); // 提交上线/调整分组上线/撤销上线的id集合
  const itsmData = ref<{
    itsm_ticket_sn: string;
    itsm_ticket_url: string;
  }>();
  const loading = ref(true);
  let interval = 0;

  watch(
    () => route.params.versionId,
    (newV) => {
      if (newV !== 'undefined') {
        loadStatus();
      }
    },
  );

  watch(approveStatus, (newV, oldV) => {
    // 轮询中 且 状态发生变化才需要刷新版本列表状态
    if (newV !== oldV && interval) {
      publishedVersionId.value = versionData.value.id;
      props.refreshVer();
    }
  });

  onMounted(async () => {
    await loadStatus();
    // 待审批和审批通过常驻显示
    if ([0, 1].includes(approveStatus.value)) {
      showStatusIdArr.value.push(Number(route.params.versionId));
    }
  });

  onUnmounted(() => {
    clearInterval(interval);
  });

  const loadStatus = debounce(async () => {
    loading.value = true;
    if (interval) {
      clearInterval(interval);
    }
    if (route.params.versionId) {
      const { spaceId, appId, versionId } = route.params;
      try {
        const resp = await versionStatusQuery(String(spaceId), Number(appId), Number(versionId));
        const {
          app,
          spec,
          spec: { itsm_ticket_sn, itsm_ticket_url, publish_time, reject_reason },
        } = resp.data;
        // 审批人
        approverList.value = spec.approver_progress;
        approveText.value = publishStatusText(spec.publish_status);
        approveType.value = app?.approve_type || '';
        rejectionReason.value = reject_reason;
        publishTime.value = publish_time;
        // itsm信息
        itsmData.value = { itsm_ticket_sn, itsm_ticket_url };
        sendData(resp.data);
        // 需要展示状态的版本
        filterShowVer();
        // 待审批/待上线状态 且 待上线为定时上线才需要轮询
        if (approveStatus.value === 0 || (approveStatus.value === 1 && publishTime.value)) {
          interval = setTimeout(loadStatus, 5000);
        }
        // 带审批/待上线状态轮询
        // if ([0, 1].includes(approveStatus.value)) {
        //   interval = setTimeout(loadStatus, 5000);
        // }
      } catch (error) {
        console.log(error);
        clearInterval(interval);
      } finally {
        loading.value = false;
      }
    }
  }, 300);

  const publishStatusText = (type: string) => {
    switch (type) {
      case 'pending_approval':
        approveStatus.value = APPROVE_TYPE.pending_approval;
        return t('待审批');
      case 'pending_publish':
        approveStatus.value = APPROVE_TYPE.pending_publish;
        return t('待上线');
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

  // 版本状态是否显示(撤销/驳回的状态刷新页面后就消失，其他状态保持展示)
  const filterShowVer = () => {
    // 更新了版本状态的id都需要展示(撤销/拒绝刷新消失)
    if (props.showStatusId > -1 && !showStatusIdArr.value.includes(props.showStatusId)) {
      showStatusIdArr.value.push(props.showStatusId);
    }
    // 如果当前版本状态为待审批/审批通过，也需要展示（常驻显示）
    if ([0, 1].includes(approveStatus.value) && !showStatusIdArr.value.includes(Number(route.params.versionId))) {
      showStatusIdArr.value.push(Number(route.params.versionId));
    }
  };

  const handleCopy = (str?: string) => {
    if (!str) return;
    copyToClipBoard(str);
    BkMessage({
      theme: 'success',
      message: t('ITSM 审批链接已复制！'),
    });
  };

  // 跳转审批页面
  const handleLinkTo = (url: string) => {
    if (url) {
      window.open(url, '_blank');
    }
  };

  const sendData = (data: any) => {
    const { spec, revision } = data;
    const releaseGroupIds = spec.scope?.groups.map((item: any) => item.id);
    const approveData = {
      status: spec.publish_status,
      time: spec.publish_time,
      type: spec.publish_type,
      groupIds: releaseGroupIds,
      memo: spec.memo,
    };
    emits('send-data', approveData, revision?.creator || '');
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
      border: 3px solid #c4c6cc;
      background-color: #f0f1f5;
    }
    &.offline {
      border: 3px solid #eeeef0;
      background-color: #979ba5;
    }
  }
  .popover-content {
    font-size: 12px;
    line-height: 16px;
    color: #4d4f56;
    .itsm-sn {
      cursor: pointer;
    }
    .itsm-content {
      display: flex;
      justify-content: flex-start;
      align-items: center;
      color: #4d4f56;
      &.em {
        color: #3a84ff;
      }
      & + .itsm-title {
        margin-top: 18px;
      }
    }
    .itsm-action {
      margin-left: 10px;
      padding-left: 10px;
      display: flex;
      align-items: center;
      border-left: 1px solid #dcdee5;
      cursor: pointer;
    }
  }
</style>
