<template>
  <div class="fields-setting-wrap">
    <div class="title">{{ $t('字段设置') }}</div>
    <bk-table
      class="fields-setting-table"
      :border="['outer']"
      :data="list"
      :max-height="400"
      :checked="checkedList"
      :is-row-select-enable="isRowSelectEnable"
      @select-all="handleSelectAll"
      @selection-change="handleSelectionChange">
      <bk-table-column :width="40" type="selection" align="center"></bk-table-column>
      <bk-table-column :label="$t('字段名')" prop="name">
        <template #default="{ row }">
          <span v-if="row.name">
            {{ row.name }}
            <bk-tag v-if="row.primary" theme="info" style="margin-left: 8px"> {{ $t('主键') }} </bk-tag>
          </span>
        </template>
      </bk-table-column>
      <bk-table-column :label="$t('显示名')" prop="alias"></bk-table-column>
      <bk-table-column :label="$t('字段类型')" prop="column_type">
        <template #default="{ row }">
          <span>{{ columnTypeMap[row.column_type as keyof typeof columnTypeMap] }}</span>
        </template>
      </bk-table-column>
    </bk-table>
    <div class="operations-btn">
      <bk-button theme="primary" @click="handleConfirm">{{ $t('确定') }}</bk-button>
      <bk-button @click="emits('close')">{{ $t('取消') }}</bk-button>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();

  interface IFieldItem {
    name: string;
    alias: string;
    column_type: string;
    primary: boolean;
  }
  const props = defineProps<{
    list: IFieldItem[];
    selectList: IFieldItem[];
  }>();
  const emits = defineEmits(['change', 'close']);

  const columnTypeMap = {
    string: t('字符串'),
    number: t('数字'),
    enum: t('枚举'),
  };

  const checkedList = ref([...props.selectList]);

  const handleSelectAll = ({ checked }: { checked: boolean }) => {
    if (checked) {
      checkedList.value = [...props.list];
    } else {
      checkedList.value = [props.list[0]];
    }
  };

  const handleSelectionChange = ({ checked, row }: { checked: boolean; row: IFieldItem }) => {
    if (checked) {
      if (!checkedList.value.find((item) => item.name === row.name)) {
        checkedList.value.push(row);
      }
    } else {
      const index = checkedList.value.findIndex((item) => item.name === row.name);
      if (index > -1) {
        checkedList.value.splice(index, 1);
      }
    }
  };

  const isRowSelectEnable = ({ isCheckAll, row }: any) => {
    if (isCheckAll) {
      return true;
    }
    if (row.primary) {
      return false;
    }
    return true;
  };

  const handleConfirm = () => {
    emits('change', checkedList.value);
    emits('close');
  };
</script>

<style scoped lang="scss">
  .fields-setting-wrap {
    padding: 12px 16px;
    background: #f5f7fa;
    border-radius: 2px;
    margin-bottom: 24px;
    .title {
      color: #313238;
      font-size: 14px;
    }
    .operations-btn {
      display: flex;
      gap: 8px;
    }
  }
  .fields-setting-table {
    margin: 8px 0 16px;
  }
</style>
