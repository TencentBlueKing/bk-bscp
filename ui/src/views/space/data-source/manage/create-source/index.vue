<template>
  <DetailLayout :name="$t('新建表格')">
    <template #content>
      <div class="create-table-content">
        <Card :title="$t('数据源类型')">
          <div class="data-source-type">
            <div
              v-for="item in dataSourceType"
              :key="item.value"
              :class="[
                'data-source-type-item',
                { disabled: item.value === 'tencent-doc', active: selectedType === item.value },
              ]">
              <div class="header">
                <i class="bk-bscp-icon icon-revoke" />
                <span class="title">{{ item.name }}</span>
              </div>
              <div class="info">{{ item.info }}</div>
            </div>
          </div>
        </Card>
        <ConnectInfo v-if="selectedType === 'mysql'" />
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import DetailLayout from '../../component/detail-layout.vue';
  import Card from '../../component/card.vue';
  import ConnectInfo from './connect-info.vue';
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();

  const selectedType = ref('mysql');

  const dataSourceType = [
    {
      name: t('MySQL 数据源'),
      value: 'mysql',
      info: t(
        '可以从 MySQL 表导入数据，自动转换 MySQL 表结构字段类型。支持开启数据同步功能，开启后，产品页面数据只读，以 MySQL 表数据为准',
      ),
    },
    {
      name: t('腾讯文档表格数据源'),
      value: 'tencent-doc',
      info: t(
        '可以从腾讯文档表格导入数据，同时需要补充表结构。支持开启数据同步功能，开启后，产品页面数据只读，以腾讯文档表格数据为准',
      ),
    },
  ];
</script>

<style scoped lang="scss">
  .create-table-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    background: #f5f7fa;
    padding: 24px 0;
  }
  .data-source-type {
    margin-top: 12px;
    display: flex;
    justify-content: space-between;
    .data-source-type-item {
      padding: 16px;
      width: 464px;
      height: 104px;
      border-radius: 2px;
      .header {
        display: flex;
        align-items: center;
        .bk-bscp-icon {
          font-size: 24px;
          margin-right: 7px;
        }
        .title {
          font-size: 14px;
        }
      }
      .info {
        margin-top: 12px;
        font-size: 12px;
        color: #979ba5;
      }
      &.active {
        color: #3a84ff;
        border: 1px solid #3a84ff;
        background: #f0f5ff;
        cursor: pointer;
      }
      &.disabled {
        color: #c4c6cc;
        cursor: not-allowed;
        border: 1px solid #eaebf0;
        background: #fafbfd;
        .info {
          color: #c4c6cc;
        }
      }
    }
  }
</style>
