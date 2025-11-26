<template>
  <div class="variable-wrap">
    <div class="head">
      <div class="close-btn" @click="emits('close')">
        <angle-down-line class="close-icon" />
      </div>
      <span class="title">{{ $t('变量') }}</span>
    </div>
    <div class="variable-content">
      <SearchInput v-model="searchValue" :clearable="false" />
      <PrimaryTable class="variable-table" :data="variableData" size="small">
        <TableColumn title="KEY" col-key="key" />
        <TableColumn :title="$t('类型')" col-key="type" />
        <TableColumn :title="$t('描述')" col-key="memo" width="120" ellipsis />
        <TableColumn :title="$t('操作')" col-key="action">
          <template #default="{ row }">
            <div class="op-btns">
              <edit-line class="icon" @click="handleEdit(row)" />
              <Del class="icon" @click="handleDelete(row)" />
            </div>
          </template>
        </TableColumn>
      </PrimaryTable>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { AngleDownLine, EditLine, Del } from 'bkui-vue/lib/icon';
  import SearchInput from '../../../../components/search-input.vue';

  const emits = defineEmits(['close']);
  const searchValue = ref('');
  const variableData = ref([
    { key: '变量1', type: '字符串', memo: '这是变量1的描述' },
    { key: '变量2', type: '数字', memo: '这是变量2的描述' },
    { key: '变量3', type: '布尔值', memo: '这是变量3的描述' },
    { key: '变量4', type: '数组', memo: '这是变量4的描述' },
    { key: '变量5', type: '对象', memo: '这是变量5的描述' },
  ]);

  const handleEdit = (row: any) => {
    console.log('Insert variable:', row);
  };
  const handleDelete = (row: any) => {
    console.log('Delete variable:', row);
  };
</script>

<style scoped lang="scss">
  .variable-wrap {
    width: 417px;
    height: 100%;
    border-radius: 4px;
    background: #f5f7fa;
    .head {
      display: flex;
      align-items: center;
      height: 40px;
      line-height: 40px;
      background: #2e2e2e;
      .close-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 30px;
        height: 40px;
        background: #478efd;
        cursor: pointer;
        .close-icon {
          color: #ffffff;
          font-size: 14px;
          transform: rotate(-90deg);
        }
      }
      .title {
        margin-left: 8px;
        font-size: 14px;
        color: #e6e6e6;
      }
    }
    .variable-content {
      padding: 16px;
      height: calc(100% - 40px);
      background: #242424;
      .search-input {
        height: 32px;
        margin-bottom: 16px;
        :deep(.bk-input) {
          border: 1px solid #63656e;
          border-radius: 2px;
          .search-input-icon {
            background: none;
          }
          .bk-input--text {
            background: none;
            color: #63656e;
            &::placeholder {
              color: #63656e;
            }
          }
        }
      }
      .variable-table {
        .op-btns {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 14px;
          .icon {
            cursor: pointer;
            &:hover {
              color: #3a84ff;
            }
          }
        }
      }
    }
  }
</style>

<style lang="scss">
  .variable-table {
    .t-table__header th {
      background: #53545c;
      color: #dcdee5 !important;
      border-color: #4a4a4a;
      &:hover {
        background: #53545c;
      }
    }
    .t-table__body td {
      background: #242424;
      color: #979ba5 !important;
      border-color: #4a4a4a;
    }
  }
</style>
