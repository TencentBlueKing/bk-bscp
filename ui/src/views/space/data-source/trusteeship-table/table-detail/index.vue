<template>
  <DetailLayout :name="t('表格详情')" :show-footer="showContent === 'table-structure'" @close="handleCloseCreate">
    <template #content>
      <div class="table-detail-wrap">
        <div class="table-detail-content">
          <div class="tab-list">
            <div
              v-for="tab in tabList"
              :key="tab.value"
              :class="['tab-item', { active: showContent === tab.value }]"
              @click="showContent = tab.value">
              {{ tab.label }}
            </div>
          </div>
          <DataPreview v-if="showContent === 'data-preview'" />
          <TableStructure v-else />
        </div>
      </div>
    </template>
    <template #footer>
      <bk-button theme="primary">{{ $t('编辑表结构') }}</bk-button>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  // import { useRoute } from 'vue-router';
  import DetailLayout from '../../component/detail-layout.vue';
  import DataPreview from './data-preview/index.vue';
  import TableStructure from './table-structure/index.vue';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const emits = defineEmits(['close']);

  const tabList = [
    {
      label: t('数据预览'),
      value: 'data-preview',
    },
    {
      label: t('表结构'),
      value: 'table-structure',
    },
  ];
  const showContent = ref('data-preview');

  // const route = useRoute();
  // const bkBizId = String(route.params.spaceId);

  const handleCloseCreate = () => {
    emits('close');
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
</style>
