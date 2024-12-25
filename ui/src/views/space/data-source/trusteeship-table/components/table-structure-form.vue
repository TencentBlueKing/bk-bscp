<template>
  <div class="table-structure-form">
    <Card :title="$t('字段设置')">
      <template v-if="isManualCreate" #suffix>
        <div class="add-fields" @click="handleAddFields">
          <Plus class="add-icon" />
          <span class="text">{{ $t('添加字段') }}</span>
        </div>
      </template>
      <FieldsTable
        :is-edit="props.isEdit"
        :has-table-data="hasTableData"
        :list="formData.columns"
        @change="handleFieldsChange" />
      <!-- <UploadFieldsTable v-else-if="filedsList.length" :list="filedsList"></UploadFieldsTable>
      <bk-exception
        v-else
        class="exception-wrap-item"
        :description="$t('请先上传文件')"
        :title="$t('暂无数据')"
        type="empty" /> -->
    </Card>
    <bk-form form-type="vertical" :model="formData">
      <Card :title="$t('基本信息')">
        <div class="basic-info-form">
          <bk-form-item :label="$t('表格名称')" property="table_name" required>
            <bk-input v-model="formData.table_name" :disabled="isEdit" @change="handleFormChange" />
          </bk-form-item>
          <bk-form-item :label="$t('表格描述')" property="table_memo">
            <bk-input v-model="formData.table_memo" @change="handleFormChange" />
          </bk-form-item>
        </div>
      </Card>
      <Card :title="$t('可见范围')">
        <bk-form-item :label="$t('选择服务')" property="visible_range" required>
          <bk-select
            v-model="formData.visible_range"
            :loading="serviceLoading"
            style="width: 464px"
            multiple
            filterable
            :placeholder="$t('请选择服务')"
            @change="handleServiceChange">
            <bk-option value="*" :label="$t('全部服务')"></bk-option>
            <bk-option v-for="service in serviceList" :key="service.id" :label="service.spec.name" :value="service.id">
            </bk-option>
          </bk-select>
        </bk-form-item>
      </Card>
    </bk-form>
  </div>
</template>

<script lang="ts" setup>
  import { ref, onMounted } from 'vue';
  import { getAppList } from '../../../../../api/index';
  import { IAppItem } from '../../../../../../types/app';
  import { IFiledsItemEditing, ILocalTableFormEditing, ILocalTableForm } from '../../../../../../types/kv-table';
  import { Plus } from 'bkui-vue/lib/icon';
  import Card from '../../component/card.vue';
  import FieldsTable from './fields-table/manual.vue';
  // import UploadFieldsTable from './fields-table/upload.vue';

  const props = defineProps<{
    bkBizId: string;
    isManualCreate: boolean;
    isEdit: boolean;
    form: ILocalTableForm;
    hasTableData?: boolean;
  }>();

  const emits = defineEmits(['change']);

  const formData = ref<ILocalTableFormEditing>({
    table_name: '',
    table_memo: '',
    visible_range: ['*'],
    columns: [],
  });
  const serviceLoading = ref(false);
  const serviceList = ref<IAppItem[]>([]);

  onMounted(() => {
    getServiceList();
  });

  const handleAddFields = () => {
    formData.value.columns.push({
      name: '',
      alias: '',
      column_type: '',
      default_value: '',
      primary: formData.value.columns.length === 0,
      not_null: false,
      unique: false,
      auto_increment: false,
      read_only: false,
      id: Date.now(),
      enum_value: [], // 枚举值设置内容
      selected: false, // 枚举值是否多选
    });
  };

  const handleFieldsChange = (val: IFiledsItemEditing[]) => {
    formData.value.columns = val;
    handleFormChange();
  };

  const getServiceList = async () => {
    serviceLoading.value = true;
    try {
      const query = {
        start: 0,
        all: true, // @todo 确认拉全量列表参数
      };
      const resp = await getAppList(props.bkBizId, query);
      serviceList.value = resp.details;
    } catch (e) {
      console.error(e);
    } finally {
      serviceLoading.value = false;
    }
  };

  const handleServiceChange = (val: string[]) => {
    if (val.length === 0) {
      formData.value.visible_range = [];
    }
    if (formData.value.visible_range[formData.value.visible_range.length - 1] === '*') {
      formData.value.visible_range = ['*'];
    } else if (formData.value.visible_range.length > 1 && formData.value.visible_range[0] === '*') {
      formData.value.visible_range = formData.value.visible_range.slice(1);
    }
    handleFormChange();
  };

  // 接口数据转表单数据
  const translateFormData = () => {
    let default_value: any;
    const columns = props.form.columns.map((item) => {
      let enum_value;
      if (item.column_type === 'enum' && item.enum_value !== '') {
        enum_value = JSON.parse(item.enum_value);
        if (enum_value.every((item: any) => typeof item === 'string')) {
          // 字符串数组，显示名和实际值按一致处理
          enum_value = enum_value.map((value: string) => {
            return {
              text: value,
              value,
            };
          });
        }
        if (item.default_value !== '' && typeof item.default_value === 'string' && item.selected) {
          // 枚举型默认值以json字符串存储 转格式
          default_value = JSON.parse(item.default_value);
        } else {
          default_value = undefined;
        }
      } else {
        enum_value = item.enum_value;
      }
      return {
        ...item,
        enum_value,
        default_value,
        id: Date.now() + item.name,
      };
    });
    formData.value = {
      table_name: props.form.table_name,
      table_memo: props.form.table_memo,
      columns: columns as IFiledsItemEditing[],
      visible_range: props.form.visible_range.length === 0 ? ['*'] : props.form.visible_range, // 如果没有权限范围，默认为全部
    };
  };

  // 表单数据转接口数据
  const handleFormChange = () => {
    const columns = formData.value.columns.map((item) => {
      let default_value;
      if (item.default_value && item.selected) {
        default_value = JSON.stringify(item.default_value);
      } else {
        default_value = item.default_value;
      }
      let enum_value;
      if (item.column_type === 'enum' && item.enum_value.length > 0) {
        enum_value = JSON.stringify(item.enum_value);
      } else {
        enum_value = '';
      }
      return {
        default_value,
        enum_value, // 枚举值设置内容
        name: item.name,
        alias: item.alias,
        primary: item.primary,
        column_type: item.column_type,
        not_null: item.not_null,
        unique: item.unique,
        read_only: item.read_only,
        auto_increment: item.auto_increment,
        selected: item.selected,
      };
    });
    const form = {
      ...formData.value,
      columns,
      visible_range: formData.value.visible_range[0] === '*' ? [] : props.form.visible_range, // 如果没有权限范围，默认为全部
    };
    emits('change', form);
  };

  defineExpose({ translateFormData });
</script>

<style scoped lang="scss">
  .add-fields {
    display: flex;
    align-items: center;
    height: 16px;
    cursor: pointer;
    .add-icon {
      border-radius: 50%;
      background-color: #3a84ff;
      color: #fff;
      margin-right: 5px;
    }
    .text {
      color: #3a84ff;
      font-size: 12px;
    }
  }

  .table-structure-form {
    .card:not(:last-child) {
      margin-bottom: 16px;
    }
  }
  .basic-info-form {
    display: flex;
    gap: 24px;
    .bk-form-item {
      flex: 1;
    }
  }

  .exception-wrap-item {
    :deep(.bk-exception-img) {
      width: 280px;
      height: 140px;
    }
    :deep(.bk-exception-title) {
      margin-top: 8px;
      font-size: 14px;
      color: #63656e;
      line-height: 22px;
    }
    :deep(.bk-exception-description) {
      margin-top: 8px;
      font-size: 12px;
      color: #979ba5;
      line-height: 20px;
    }
  }
</style>
