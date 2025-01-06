<template>
  <bk-form-item :label="$t('表格配置')" property="managed_table_id">
    <div class="source-select">
      <div :class="['source-item', { active: sourceType === 'trusteeship' }]" @click="sourceType = 'trusteeship'">
        {{ $t('托管表格配置') }}
      </div>
      <div :class="['source-item', { active: sourceType === 'external' }]">
        {{ $t('关联外部数据源') }}
      </div>
    </div>
    <bk-select
      v-model="tableForm.managed_table_id"
      id-key="id"
      display-key="name"
      class="table-select"
      enable-virtual-render
      :popover-options="{ theme: 'light bk-select-popover table-selector-popover' }"
      :list="tableList"
      :placeholder="tableSelectPlaceholder"
      :loading="loading"
      @change="emits('change', tableForm)">
      <template #extension>
        <div class="create-operation" @click="handleToCreateOrEdit('create')">
          <plus />
          <div class="content">{{ t('新建托管表格') }}</div>
        </div>
      </template>
      <template #virtualScrollRender="{ item }">
        <div class="name-wrapper">
          <span class="text">{{ item.name }}</span>
          <span class="edit-icon" @click.stop="handleToCreateOrEdit('edit', item)">
            <edit-line />
          </span>
        </div>
      </template>
    </bk-select>
  </bk-form-item>
  <dataPreview
    v-if="tableForm.managed_table_id"
    :bk-biz-id="bkBizId"
    :table-form="tableForm"
    @change="handlePreviewConditionChange" />
</template>

<script lang="ts" setup>
  import { ref, computed, onMounted } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { EditLine, Plus } from 'bkui-vue/lib/icon';
  import { getLocalTableList } from '../../../../../../../../../api/kv-table';
  import { ILocalTableItem, IDataCleanItem, IConfigTableForm } from '../../../../../../../../../../types/kv-table';
  import { IConfigKvEditParams } from '../../../../../../../../../../types/config';
  import dataPreview from './data-preview/index.vue';

  const { t } = useI18n();
  const router = useRouter();

  const props = defineProps<{
    bkBizId: string;
    config: IConfigKvEditParams;
  }>();

  const emits = defineEmits(['change']);

  const sourceType = ref('trusteeship');
  const loading = ref(false);
  const tableList = ref<{ id: number; name: string }[]>([]);
  const tableForm = ref<IConfigTableForm>({
    managed_table_id: undefined,
    filter_condition: {},
    filter_fields: [],
  });

  const tableSelectPlaceholder = computed(() => {
    return sourceType.value === 'trusteeship' ? t('请选择表格') : t('请选择数据源');
  });

  onMounted(async () => {
    await getTableList();
    tableForm.value = {
      managed_table_id: props.config.managed_table_id || undefined,
      filter_condition: props.config.filter_condition || {},
      filter_fields: props.config.filter_fields || [],
    };
  });

  const getTableList = async () => {
    try {
      loading.value = true;
      const params = {
        start: 0,
        all: true,
      };
      const res = await getLocalTableList(props.bkBizId, params);
      tableList.value = res.details.map((item: ILocalTableItem) => {
        return {
          id: item.id,
          name: item.spec.table_name,
        };
      });
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleToCreateOrEdit = (type: string, table?: { id: number; name: string }) => {
    let routeData;
    if (type === 'create') {
      routeData = router.resolve({
        name: 'create-table-structure',
        params: { bizId: props.bkBizId },
      });
    } else {
      routeData = router.resolve({
        name: 'edit-table-structure',
        params: { bizId: props.bkBizId, id: table!.id },
        query: {
          name: table!.name,
        },
      });
    }
    window.open(routeData!.href, '_blank');
  };

  const handlePreviewConditionChange = (filter_fields: string[], labels_and: IDataCleanItem[]) => {
    tableForm.value = {
      ...tableForm.value,
      filter_fields,
      filter_condition: labels_and ? { labels_and } : {},
    };
    emits('change', tableForm.value);
  };
</script>

<style scoped lang="scss">
  .source-select {
    display: flex;
    .source-item {
      width: 214px;
      height: 26px;
      background: #ffffff;
      border: 1px solid #c4c6cc;
      border-radius: 0 2px 2px 0;
      text-align: center;
      line-height: 26px;
      font-size: 12px;
      color: #63656e;
      cursor: pointer;
      &:first-child {
        border-right: none;
        border-radius: 2px 0 0 2px;
      }
      &.active {
        background: #e1ecff;
        border: 1px solid #3a84ff;
        color: #3a84ff;
      }
    }
  }

  .table-select {
    width: 428px;
    margin-top: 12px;
  }

  .name-wrapper {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    .name {
      flex: 0 1 auto;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .edit-icon {
      font-size: 16px;
      color: #979ba5;
      display: none;
    }
    &:hover {
      .edit-icon {
        display: block;
      }
    }
  }
  .create-operation {
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    height: 33px;
    padding: 0 12px;
    width: 100%;
    color: #2b353e;
    span {
      font-size: 16px;
      margin-right: 3px;
    }
  }
</style>
