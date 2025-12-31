<template>
  <div class="op-content">
    <bk-button theme="primary" :disabled="count === 0" @click="emits('click', 'start')">{{ $t('批量启动') }}</bk-button>
    <bk-button :disabled="count === 0" @click="emits('click', 'stop')">{{ $t('批量停止') }}</bk-button>
    <bk-button :disabled="count === 0" @click="emits('click', 'issue')">{{ $t('批量配置下发') }}</bk-button>
    <bk-popover
      ref="buttonRef"
      trigger="click"
      placement="bottom"
      theme="light process-op-popover"
      :arrow="false"
      width="80"
      @after-show="isPopoverOpen = true"
      @after-hidden="isPopoverOpen = false">
      <bk-button :class="['more-op-btn', { 'popover-open': isPopoverOpen }]" :disabled="count === 0">
        {{ $t('更多') }}<angle-down class="angle-icon" />
      </bk-button>
      <template #content>
        <div class="more-list">
          <div
            :class="['more-item', { disabled: count === 0 }]"
            v-for="item in moreOperation"
            :key="item.value"
            @click="handleClick(item.value)">
            {{ item.label }}
          </div>
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

  const emits = defineEmits(['click']);
  defineProps<{
    count: number;
  }>();

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
      value: 'kill',
    },
    {
      label: t('托管'),
      value: 'register',
    },
    {
      label: t('取消托管'),
      value: 'unregister',
    },
  ];
  const buttonRef = ref();
  const isPopoverOpen = ref(false);

  const handleClick = (op: string) => {
    emits('click', op);
    buttonRef.value.hideHandler();
  };
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
      &.disabled {
        color: #c4c6cc;
        cursor: not-allowed;
      }
    }
  }
</style>

<style lang="scss">
  .process-op-popover {
    padding: 0 !important;
  }
</style>
