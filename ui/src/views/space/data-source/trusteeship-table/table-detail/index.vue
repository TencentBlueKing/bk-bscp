<template>
  <DetailLayout
    :name="t('表格详情')"
    :show-footer="showContent === 'trusteeship-table-structure-preview'"
    :suffix="name"
    @close="geToTable">
    <template #content>
      <div class="table-detail-wrap">
        <div class="table-detail-content">
          <div class="tab-list">
            <div
              v-for="tab in tabList"
              :key="tab.value"
              :class="['tab-item', { active: showContent === tab.value }]"
              @click="handleChangeView(tab.value)">
              {{ tab.label }}
            </div>
          </div>
          <router-view></router-view>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary">{{ $t('编辑表结构') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useRouter, useRoute } from 'vue-router';
  import DetailLayout from '../../component/detail-layout.vue';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const router = useRouter();
  const route = useRoute();

  const tableId = ref(route.params.id);
  const spaceId = ref(String(route.params.spaceId));

  const name = ref(String(route.query.name));

  const tabList = [
    {
      label: t('数据预览'),
      value: 'trusteeship-table-data-preview',
    },
    {
      label: t('表结构'),
      value: 'trusteeship-table-structure-preview',
    },
  ];
  const showContent = ref('trusteeship-table-data-preview');

  const handleChangeView = (value: string) => {
    showContent.value = value;
    router.push({ name: value, params: { spaceId: spaceId.value, id: tableId.value }, query: { name: name.value } });
  };

  const geToTable = () => {
    router.push({ name: 'trusteeship-table-list', params: { spaceId: spaceId.value } });
  };
</script>

<style scoped lang="scss">
  .table-detail-wrap {
    height: 100%;
    background-color: #f5f7fa;
    padding: 12px 24px 0;
    .table-detail-content {
      height: 100%;
      .tab-list {
        display: flex;
        gap: 8px;
        .tab-item {
          min-width: 90px;
          height: 42px;
          line-height: 42px;
          text-align: center;
          background: #eaebf0;
          border-radius: 4px 4px 0 0;
          color: #63656e;
          cursor: pointer;
          &.active {
            background: #ffffff;
            color: #3a84ff;
          }
        }
      }
    }
  }
  .operation-btns {
    width: calc(100% - 48px);
  }
</style>
