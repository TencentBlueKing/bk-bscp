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
      <bk-loading color="#242424" :loading="loading">
        <PrimaryTable
          class="variable-table"
          :data="variableList"
          size="small"
          row-key="key"
          hover
          @row-click="handleRowClick">
          <TableColumn :title="$t('名称')" col-key="key" />
          <TableColumn :title="$t('变量')" col-key="value" />
        </PrimaryTable>
      </bk-loading>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { AngleDownLine } from 'bkui-vue/lib/icon';
  import { getConfigTemplateVariable } from '../../../../api/config-template';
  import SearchInput from '../../../../components/search-input.vue';
  import { copyToClipBoard } from '../../../../utils';
  import { useI18n } from 'vue-i18n';
  import BkMessage from 'bkui-vue/lib/message';

  interface IVariableItem {
    key: string;
    value: string;
  }

  const { t } = useI18n();

  const emits = defineEmits(['close']);
  const props = defineProps<{
    bkBizId: string;
  }>();

  const searchValue = ref('');
  const variableList = ref<IVariableItem[]>([]);
  const loading = ref(false);

  onMounted(() => {
    loadVariableList();
  });

  const loadVariableList = async () => {
    try {
      loading.value = true;
      const res = await getConfigTemplateVariable(props.bkBizId);
      variableList.value = res.config_template_variables;
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleRowClick = ({ row }: { row: IVariableItem }) => {
    copyToClipBoard(row.value);
    BkMessage({
      theme: 'success',
      message: t('变量值已复制'),
    });
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
    .t-table__body tr {
      td {
        background: #242424;
        color: #979ba5 !important;
        border-color: #4a4a4a;
      }
      &:hover {
        cursor: pointer;
        td {
          background: #2e2e2e;
        }
      }
    }
  }
</style>
