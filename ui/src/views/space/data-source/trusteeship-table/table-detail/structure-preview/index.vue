<template>
  <div class="table-structure-wrap">
    <section class="fields-setting">
      <div class="title">{{ $t('字段设置') }}</div>
      <ViewTable :fileds-list="formData.columns" />
    </section>
    <section class="basic-info">
      <div class="title">{{ $t('基本信息') }}</div>
      <div class="info-content">
        <div class="info-item">
          <div class="info-title">{{ $t('表格名称') }}</div>
          <div class="content">{{ formData.table_name }}</div>
        </div>
        <div class="info-item">
          <div class="info-title">{{ $t('表格描述') }}</div>
          <div class="content">{{ formData.table_memo || '--' }}</div>
        </div>
      </div>
    </section>
    <section class="visible-range">
      <div class="title">{{ $t('可见范围') }}</div>
      <div class="info-item">
        <div class="info-title">{{ $t('服务') }}</div>
        <div class="content">{{ '全部服务' }}</div>
      </div>
    </section>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { useRoute } from 'vue-router';
  import { getTableStructure } from '../../../../../../api/kv-table';
  import { ILocalTableFormEditing, IFiledsItemEditing } from '../../../../../../../types/kv-table';
  import ViewTable from './view-table.vue';

  const route = useRoute();

  const spaceId = ref(String(route.params.spaceId));
  const id = ref(Number(route.params.id));

  const formData = ref<ILocalTableFormEditing>({
    table_name: '',
    table_memo: '',
    visible_range: ['*'],
    columns: [],
  });

  onMounted(() => {
    getStructure();
  });

  const getStructure = async () => {
    try {
      const res = await getTableStructure(spaceId.value, id.value);
      const columns = res.details.spec.columns.map((item: any, index: number) => {
        let default_value: string | string[] | undefined;
        let enum_value;
        if (item.column_type === 'enum' && item.enum_value !== '') {
          enum_value = JSON.parse(item.enum_value);
          if (enum_value.every((item: any) => typeof item === 'string')) {
            // 字符串数组，显示名和实际值按一致处理
            enum_value = enum_value.map((value: string) => {
              return {
                label: value,
                value,
              };
            });
          }
        } else {
          enum_value = item.enum_value;
        }

        if (item.column_type === 'enum') {
          const isMultiSelect = item.selected; // 是否多选
          const hasDefaultValue = !!item.default_value;

          if (isMultiSelect) {
            // 多选情况下，解析为数组或赋值为空数组
            default_value = hasDefaultValue ? JSON.parse(item.default_value as string) : [];
          } else {
            // 单选情况下，直接赋值或设置为 undefined select组件tag模式设置空字符串会有空tag
            default_value = hasDefaultValue ? item.default_value : undefined;
          }
        } else {
          // 非枚举类型直接赋值
          default_value = item.default_value;
        }
        return {
          ...item,
          enum_value,
          default_value,
          id: Date.now() + index,
        };
      });
      formData.value = {
        table_name: res.details.spec.table_name,
        table_memo: res.details.spec.table_memo,
        columns: columns as IFiledsItemEditing[],
        visible_range: res.details.spec.visible_range.length === 0 ? ['*'] : res.details.spec.visible_range, // 如果没有权限范围，默认为全部
      };
    } catch (error) {
      console.error(error);
    }
  };
</script>

<style scoped lang="scss">
  .table-structure-wrap {
    display: flex;
    flex-direction: column;
    gap: 32px;
    padding: 16px 24px;
    height: calc(100% - 42px);
    overflow: auto;
    background-color: #fff;
    .title {
      font-weight: 700;
      font-size: 14px;
      color: #63656e;
      margin-bottom: 16px;
    }
    .info-content {
      display: flex;
    }
    .info-item {
      width: 200px;
      font-size: 12px;
      .info-title {
        color: #979ba5;
        margin-bottom: 4px;
      }
      .content {
        color: #313238;
      }
    }
  }
</style>
