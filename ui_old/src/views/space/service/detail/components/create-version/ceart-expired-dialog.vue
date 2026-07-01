<template>
  <bk-dialog
    :is-show="show"
    ref="dialog"
    ext-cls="confirm-dialog"
    footer-align="center"
    :cancel-text="$t('再想想')"
    :confirm-text="$t('继续生成版本')"
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
        {{ $t('证书即将过期') }}
      </div>
    </template>
    <div class="record-hd">
      <span>{{ $t('30天内过期的证书列表') }}</span>
    </div>
    <div class="record-bd">
      <div class="record-bd__table">
        <div class="table-tr">
          <div class="table-th">{{ $t('证书名称') }}</div>
          <div class="table-th">{{ $t('剩余时间') }}</div>
          <div class="table-th time">{{ $t('过期时间') }}</div>
        </div>
        <div class="table-tr" v-for="item in dialogData" :key="item.id">
          <div class="table-td">{{ item.name || '--' }}</div>
          <div class="table-td">{{ $t('{n} 天', { n: item.remainingDays }) }}</div>
          <div class="table-td time">{{ item.expirationTime }}</div>
        </div>
      </div>
    </div>
  </bk-dialog>
</template>

<script setup lang="ts">
  import { ExclamationCircleShape } from 'bkui-vue/lib/icon';
  import { IExpiredCert } from '../../../../../../../types/config';

  const emits = defineEmits(['update:show', 'confirm']);
  withDefaults(
    defineProps<{
      show: boolean;
      dialogData: IExpiredCert[];
    }>(),
    {},
  );

  const handleClose = () => {
    emits('update:show', false);
  };
  const handleConfirm = () => {
    emits('update:show', false);
    setTimeout(() => {
      emits('confirm');
    }, 300);
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
    max-height: 300px;
    overflow: auto;
    .table-tr {
      min-height: 42px;
      display: flex;
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
      &.time {
        width: 50%;
      }
    }
  }
</style>
