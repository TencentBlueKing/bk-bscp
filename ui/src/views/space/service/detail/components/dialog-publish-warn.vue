<template>
  <bk-dialog
    :is-show="show"
    ref="dialog"
    ext-cls="confirm-dialog"
    footer-align="center"
    :cancel-text="$t('再想想')"
    :width="dialogType === 'confirm' ? '480' : '640'"
    :confirm-text="dialogType === 'confirm' ? $t('我知道了') : $t('继续上线')"
    :dialog-type="dialogType === 'confirm' ? 'confirm' : 'operation'"
    :close-icon="true"
    :show-mask="true"
    :quick-close="false"
    :multi-instance="false"
    @confirm="handleConfirm"
    @closed="handleClose">
    <template #header>
      <div class="tip-icon__wrap">
        <exclamation-circle-shape class="tip-icon" />
      </div>
      <div class="headline">
        {{ dialogType === 'confirm' ? $t('当前服务有正在上线的任务，请稍后尝试') : $t('高频上线风险提示') }}
      </div>
    </template>
    <div v-if="dialogType === 'confirm'" class="content-info">
      {{ $t('正在上线的版本：') }}
      <span class="content-info__bd">
        {{ dialogData.version_name }}
      </span>
      <share class="share" @click="handleLinkTo($event, dialogData.release_id)" />
    </div>
    <template v-else>
      <div class="content-info is-special">
        <template v-if="locale !== 'en'">
          距上次版本上线<span class="content-info--em">不到 2 小时</span>
          ，请确保当前情况确实需要上线版本，避免过于频繁的上线操作可能带来的潜在风险
        </template>
        <template v-else>
          The time since the last version release <span class="content-info--em">is under 2 hours</span>. Please confirm
          that the current deployment is essential to mitigate potential risks from frequent updates
        </template>
      </div>
      <div class="record-hd">
        <span>{{ $t('近三次版本上线记录') }}</span>
        <bk-link theme="primary" target="_blank" @click="handleLinkTo($event, 'record-table')">
          <share class="share" />{{ $t('查看全部上线记录') }}
        </bk-link>
      </div>
      <div class="record-bd">
        <div class="record-bd__table">
          <div class="table-tr">
            <div class="table-th">{{ $t('上线时间') }}</div>
            <div class="table-th">{{ $t('上线版本') }}</div>
            <div class="table-th">{{ $t('上线范围') }}</div>
            <div class="table-th">{{ $t('操作人') }}</div>
          </div>
          <div class="table-tr" v-for="(item, index) in dialogData.publish_record" :key="index">
            <div class="table-td">
              {{ item.final_approval_time ? convertTime(item.final_approval_time, 'local') : '--' }}
            </div>
            <div class="table-td">{{ item.name || '--' }}</div>
            <div class="table-td">{{ item.fully_released ? '全部实例' : versionScope(item.scope.groups) }}</div>
            <div class="table-td"><bk-user-display-name :user-id="item.creator" /></div>
          </div>
        </div>
      </div>
    </template>
  </bk-dialog>
</template>

<script setup lang="ts">
  import { computed } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import { ExclamationCircleShape, Share } from 'bkui-vue/lib/icon';
  import { IPublishData } from '../../../../../../types/service';
  import { convertTime } from '../../../../../utils';
  import { APPROVE_STATUS } from '../../../../../constants/record';
  import { useI18n } from 'vue-i18n';

  const emits = defineEmits(['update:show', 'confirm']);

  const props = withDefaults(
    defineProps<{
      show: boolean;
      dialogData: IPublishData;
    }>(),
    {},
  );

  const route = useRoute();
  const router = useRouter();

  const { locale } = useI18n();

  const dialogType = computed(() => (props.dialogData.is_publishing ? 'confirm' : 'other'));

  const handleLinkTo = ($event: MouseEvent, target: string | number) => {
    $event.preventDefault();
    const param = target === 'record-table' ? APPROVE_STATUS.already_publish : target;
    if (target === 'record-table') {
      // 新开页面跳转操作记录列表 且查看当前服务所有已上线版本
      const url = router.resolve({
        name: 'records-app',
        query: {
          status: param,
        },
        params: {
          appId: route.params.appId,
        },
      }).href;
      window.open(url, '_blank');
    } else {
      // 当前页跳转配置页面正在发布的版本
      const url = router.resolve({
        name: 'service-config',
        params: {
          versionId: param,
        },
      }).href;
      window.location.href = url;
    }
  };

  const versionScope = <T extends { spec: { name?: string } }>(data: T[]) => {
    return data.map((item: T) => item.spec.name).join(';');
  };
  const handleClose = () => {
    emits('update:show', false);
  };
  const handleConfirm = () => {
    emits('confirm', dialogType.value === 'other');
  };
</script>

<style lang="scss" scoped>
  :deep(.confirm-dialog) {
    .bk-modal-body {
      padding-bottom: 0;
    }
    .bk-modal-content {
      padding: 0 32px;
      height: auto;
      max-height: none;
      min-height: auto;
      border-radius: 2px;
    }
    .bk-modal-footer {
      position: relative;
      padding: 24px 0;
      height: auto;
      border: none;
    }
    .bk-dialog-footer .bk-button {
      min-width: 88px;
    }
  }
  .tip-icon__wrap {
    margin: 0 auto;
    width: 42px;
    height: 42px;
    position: relative;
    &::after {
      content: '';
      position: absolute;
      z-index: -1;
      top: 50%;
      left: 50%;
      transform: translate3d(-50%, -50%, 0);
      width: 30px;
      height: 30px;
      border-radius: 50%;
      background-color: #ff9c01;
    }
    .tip-icon {
      font-size: 42px;
      line-height: 42px;
      vertical-align: middle;
      color: #ffe8c3;
    }
  }
  .headline {
    margin-top: 16px;
    text-align: center;
  }
  .content-info {
    margin-top: 4px;
    padding: 12px 16px;
    font-size: 12px;
    line-height: 20px;
    color: #63656e;
    background-color: #f5f6fa;
    &__bd {
      color: #313238;
    }
    &--em {
      font-weight: 700;
      color: #ff9c01;
    }
    &.is-special {
      font-size: 14px;
      line-height: 22px;
    }
  }
  .share {
    margin-left: 9px;
    font-size: 12px;
    color: #3a84ff;
    vertical-align: middle;
    cursor: pointer;
  }
  .record-hd {
    position: relative;
    margin-top: 16px;
    padding-left: 10px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 14px;
    line-height: 22px;
    color: #313238;
    &::after {
      content: '';
      position: absolute;
      left: 0;
      top: 50%;
      transform: translateY(-50%);
      width: 4px;
      height: 16px;
      border-radius: 0 2px 2px 0;
      background-color: #699df4;
    }
    .share {
      margin: 0 5px 0 0;
    }
  }
  // 表格
  .record-bd {
    margin-top: 8px;
  }
  .record-bd__table {
    .table-tr {
      min-height: 42px;
      display: flex;
      justify-content: center;
    }
    .table-th {
      display: flex;
      justify-content: flex-start;
      align-items: center;
      font-size: 12px;
      border-bottom: 1px solid #e1e2e9;
      color: #313238;
      background-color: #f0f1f5;
    }
    .table-td {
      // padding: 4px 8px;
      display: flex;
      justify-content: flex-start;
      align-items: center;
      font-size: 12px;
      border-bottom: 1px solid #e1e2e9;
      color: #63656e;
      background-color: #fff;
    }
    .table-th,
    .table-td {
      padding: 4px 0 4px 16px;
      width: 25%;
      word-wrap: break-word;
      word-break: break-all;
      &:last-child {
        padding-right: 16px;
      }
    }
  }
</style>
