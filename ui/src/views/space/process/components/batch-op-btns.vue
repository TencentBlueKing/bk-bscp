<template>
  <div class="op-content">
    <bk-button theme="primary" @click="emits('start')">{{ $t('批量启动') }}</bk-button>
    <bk-button @click="emits('stop')">{{ $t('批量停止') }}</bk-button>
    <bk-button>{{ $t('批量配置下发') }}</bk-button>
    <bk-popover
      ref="buttonRef"
      trigger="click"
      placement="bottom"
      theme="light process-op-popover"
      :arrow="false"
      width="80"
      @after-show="isPopoverOpen = true"
      @after-hidden="isPopoverOpen = false">
      <bk-button :class="['more-op-btn', { 'popover-open': isPopoverOpen }]">
        {{ $t('更多') }}<angle-down class="angle-icon" />
      </bk-button>
      <template #content>
        <div class="more-list">
          <div class="more-item" v-for="item in moreOperation" :key="item.value">{{ item.label }}</div>
        </div>
      </template>
    </bk-popover>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { AngleDown } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const emits = defineEmits(['start', 'stop']);

  const moreOperation = [
    {
      label: t('重启'),
      value: 'restart',
    },
    {
      label: t('重载'),
      value: 'reload',
    },
    {
      label: t('强制停止'),
      value: 'force_stop',
    },
    {
      label: t('托管'),
      value: 'hosted',
    },
    {
      label: t('取消托管'),
      value: 'unhosted',
    },
  ];
  const buttonRef = ref();
  const isPopoverOpen = ref(false);
</script>

<style scoped lang="scss">
  .op-content {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .more-op-btn {
    width: 80px;
    &.popover-open {
      .angle-icon {
        transform: rotate(-180deg);
      }
    }
  }
  .more-list {
    .more-item {
      padding: 0 12px;
      height: 32px;
      line-height: 32px;
      color: #63656e;
      font-size: 12px;
      cursor: pointer;
      &:hover {
        background: #f5f7fa;
      }
    }
  }
</style>

<style lang="scss">
  .process-op-popover {
    padding: 0 !important;
  }
</style>
