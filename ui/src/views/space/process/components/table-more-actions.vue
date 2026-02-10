<template>
  <bk-popover :is-show="isShow" trigger="manual" theme="light" placement="bottom" :arrow="false">
    <div class="more-actions" @mouseenter="handleEnter" @mouseleave="handleLeave">
      <Ellipsis class="ellipsis-icon" />
    </div>
    <template #content>
      <ul class="dropdown-ul" @mouseenter="handleEnter" @mouseleave="handleLeave">
        <template v-for="item in operations" :key="item.id">
          <bk-popover :disabled="item.enabled">
            <li :class="getLiClass(item)" @click="handleClick(item)">
              <span>{{ item.name }}</span>
            </li>
            <template #content>
              <span
                v-if="item.reason === 'CMD_NOT_CONFIGURED'"
                class="no-cmd-content"
                @mouseenter="handleEnter"
                @mouseleave="handleLeave"
                @click="emits('operation', 'link')">
                {{ $t('尚未配置操作命令') }}
                <span class="primary">
                  {{ $t('前往 BKCC 配置') }}
                  <Share />
                </span>
              </span>
              <span v-else>
                {{ PROCESS_BUTTON_DISABLED_TIPS[item.reason as keyof typeof PROCESS_BUTTON_DISABLED_TIPS] }}
              </span>
            </template>
          </bk-popover>
        </template>
      </ul>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref, computed } from 'vue';
  import { Ellipsis, Share } from 'bkui-vue/lib/icon';
  import { PROCESS_BUTTON_DISABLED_TIPS } from '../../../../constants/process';

  interface operationType {
    name: string;
    id: string;
    enabled: boolean;
    reason: string;
  }

  const props = defineProps<{
    operationList: { name: string; id: string }[];
    enabled: Record<
      string,
      {
        enabled: boolean;
        reason: string;
      }
    >;
  }>();

  const emits = defineEmits(['operation']);

  const isShow = ref(false);

  let hideTimer: number | null = null;

  const handleEnter = () => {
    if (hideTimer) {
      clearTimeout(hideTimer);
      hideTimer = null;
    }
    isShow.value = true;
  };

  const handleLeave = () => {
    hideTimer = window.setTimeout(() => {
      isShow.value = false;
    }, 300);
  };

  const operations = computed(() => {
    return props.operationList.map((item) => ({
      ...item,
      enabled: props.enabled[item.id].enabled,
      reason: props.enabled[item.id].reason,
    }));
  });

  const getLiClass = (operation: operationType) => {
    return ['dropdown-li', { disabled: !operation.enabled }];
  };

  const handleClick = (operation: operationType) => {
    if (!operation.enabled) return;
    emits('operation', operation.id);
    isShow.value = false;
  };
</script>

<style scoped lang="scss">
  .more-actions {
    display: flex;
    align-items: center;
    justify-content: center;
    margin-left: 8px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    cursor: pointer;
    &:hover {
      background: #dcdee5;
      color: #3a84ff;
    }
    .ellipsis-icon {
      font-size: 16px;
      transform: rotate(90deg);
      cursor: pointer;
    }
  }

  .dropdown-ul {
    margin: -12px;
    font-size: 12px;
    .dropdown-li {
      padding: 0 12px;
      min-width: 68px;
      font-size: 12px;
      line-height: 32px;
      color: #4d4f56;
      cursor: pointer;
      &.disabled {
        color: #c4c6cc;
        cursor: not-allowed;
      }
      &:hover {
        background: #f5f7fa;
      }
    }
  }
  .no-cmd-content {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .primary {
    display: flex;
    align-items: center;
    gap: 4px;
    color: #3a84ff;
    cursor: pointer;
  }
</style>
