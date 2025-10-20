<template>
  <bk-popover ref="opPopRef" theme="light" placement="bottom-start" :arrow="false">
    <div class="more-actions">
      <Ellipsis class="ellipsis-icon" />
    </div>
    <template #content>
      <ul class="dropdown-ul">
        <li class="dropdown-li" v-for="item in operationList" :key="item.name" @click="item.click()">
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

  const { t } = useI18n();

  const opPopRef = ref();

  const operationList = [
    {
      name: t('重启'),
      click: () => {},
    },
    {
      name: t('重载'),
      click: () => {},
    },
    {
      name: t('强制停止'),
      click: () => {},
    },
    {
      name: t('托管'),
      click: () => {},
    },
    {
      name: t('取消托管'),
      click: () => {},
    },
    {
      name: t('查看进程配置'),
      click: () => {},
    },
  ];
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
      .no-perm {
        color: #c4c6cc;
        cursor: not-allowed;
      }
      &:hover {
        background: #f5f7fa;
      }
    }
  }
</style>
