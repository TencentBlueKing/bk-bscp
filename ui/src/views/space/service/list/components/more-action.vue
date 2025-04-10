<template>
  <bk-popover ref="opPopRef" theme="light" placement="bottom-end" :arrow="false">
    <div class="more-actions">
      <Ellipsis class="ellipsis-icon" />
    </div>
    <template #content>
      <ul class="dropdown-ul">
        <li class="dropdown-li" v-for="item in operationList" :key="item.name" @click="item.click()">
          {{ item.name }}
        </li>
      </ul>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import { IAppItem } from '../../../../../../types/app';
  import { useRouter } from 'vue-router';

  const { t } = useI18n();
  const router = useRouter();

  const props = defineProps<{
    app: IAppItem;
    spaceId: string;
  }>();
  const emits = defineEmits(['delete', 'edit']);
  const opPopRef = ref();

  const operationList = [
    {
      name: t('客户端统计'),
      click: () => handleJump('client-statistics'),
    },
    {
      name: t('编辑基本属性'),
      click: () => {
        opPopRef.value.hide();
        emits('edit');
      },
    },
    {
      name: t('配置示例'),
      click: () => handleJump('configuration-example'),
    },
    {
      name: t('操作记录'),
      click: () => handleJump('records-app'),
    },
    {
      name: t('删除'),
      click: () => {
        opPopRef.value.hide();
        emits('delete');
      },
    },
  ];

  const handleJump = (name: string) => {
    const routeData = router.resolve({
      name,
      params: { spaceId: props.spaceId, appId: props.app.id },
    });
    window.open(routeData.href, '_blank');
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
      &:hover {
        background: #f5f7fa;
      }
    }
  }
</style>
