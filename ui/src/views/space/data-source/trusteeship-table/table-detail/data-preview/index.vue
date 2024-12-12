<template>
  <div class="data-preview-wrap">
    <SheetList :list="dataList" :view-sheet="viewSheet" @change="viewSheet = $event" />
    <div class="sheet-content">
      <div class="content-header">
        <div class="head-left">
          <div class="sheet-name">{{ viewSheet }}</div>
          <bk-button theme="primary">{{ $t('编辑数据') }}</bk-button>
          <bk-button>{{ $t('导入') }}</bk-button>
          <bk-button>{{ $t('导出') }}</bk-button>
        </div>
        <div class="head-right">
          <bk-input class="search-input">
            <template #suffix>
              <Search class="search-input-icon" />
            </template>
          </bk-input>
          <bk-select class="select-input" v-model="selectedValue" auto-focus filterable :placeholder="$t('常用查询')">
            <bk-option v-for="(item, index) in selectDataSource" :id="item.value" :key="index" :name="item.label" />
          </bk-select>
          <div class="search-type-wrap">
            <div :class="['search-type-item', { active: searchType === 'basics' }]">{{ $t('基础查询') }}</div>
            <div :class="['search-type-item', { active: searchType === 'advanced' }]">{{ $t('高级查询') }}</div>
          </div>
        </div>
      </div>
      <Table />
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { Search } from 'bkui-vue/lib/icon';
  import SheetList from './sheet-list.vue';
  import Table from './table.vue';

  const dataList = ref([{ name: 'Sheet1' }, { name: 'Sheet2' }, { name: 'Sheet3' }]);
  const selectDataSource = ref([
    {
      value: 'climbing',
      label: '爬山',
    },
    {
      value: { a: 123 },
      label: '跑步',
    },
    {
      value: { b: 456 },
      label: '未知',
    },
  ]);
  const selectedValue = ref('');
  const searchType = ref('basics');

  const viewSheet = ref('Sheet1');
</script>

<style scoped lang="scss">
  .data-preview-wrap {
    display: flex;
    padding: 8px 0;
    background-color: #fff;
    height: calc(100% - 42px);
    .sheet-content {
      flex: 1;
      padding: 24px 24px 0 24px;
      .content-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 16px;
      }
      .head-left {
        display: flex;
        align-items: center;
        .sheet-name {
          width: 79px;
          height: 32px;
          line-height: 32px;
          text-align: center;
          background: #f0f1f5;
          border-radius: 16px;
          font-weight: 700;
          font-size: 14px;
          color: #63656e;
          margin-right: 16px;
        }
        .bk-button:not(:last-child) {
          margin-right: 8px;
        }
      }
      .head-right {
        display: flex;
        gap: 8px;
        .search-input {
          width: 320px;
          .search-input-icon {
            padding-right: 10px;
            color: #979ba5;
            background: #ffffff;
          }
        }
        .select-input {
          width: 120px;
        }
        .search-type-wrap {
          display: flex;
          padding: 4px;
          background: #f0f1f5;
          height: 32px;
          .search-type-item {
            min-width: 72px;
            height: 24px;
            line-height: 24px;
            text-align: center;
            font-size: 12px;
            border-radius: 2px;
            color: #63656e;
            cursor: pointer;
            &.active {
              background: #fff;
              color: #3a84ff;
            }
          }
        }
      }
    }
  }
</style>
