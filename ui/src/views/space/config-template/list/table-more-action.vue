<template>
  <bk-popover ref="opPopRef" theme="light" placement="bottom-start" :arrow="false">
    <div class="more-actions">
      <Ellipsis class="ellipsis-icon" />
    </div>
    <template #content>
      <ul class="dropdown-ul">
        <li :class="getLiClass(item.id)" v-for="item in operationList" :key="item.name" @click="handleClick(item.id)">
          <span>{{ item.name }}</span>
        </li>
      </ul>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import { IProcessTableAction } from '../../../../../types/process';

  const { t } = useI18n();

  const emits = defineEmits(['click', 'kill']);
  const props = defineProps<{
    actions: IProcessTableAction;
  }>();

  const opPopRef = ref();

  const operationList = [
    {
      name: t('重启'),
      id: 'restart',
    },
    {
      name: t('重载'),
      id: 'reload',
    },
    {
      name: t('强制停止'),
      id: 'kill',
    },
    {
      name: t('托管'),
      id: 'register',
    },
    {
      name: t('取消托管'),
      id: 'unregister',
    },
    {
      name: t('查看进程配置'),
      id: 'viewConfig',
    },
  ];

  const getLiClass = (id: string) => {
    return ['dropdown-li', { disabled: !props.actions[id as keyof typeof props.actions] }];
  };

  const handleClick = (id: string) => {
    if (!props.actions[id as keyof typeof props.actions]) return;
    if (id === 'kill') {
      emits('kill');
    } else {
      emits('click', id);
    }
    opPopRef.value.hide();
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
</style>
