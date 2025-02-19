<template>
  <div class="import-form-local-wrap">
    <Card :title="$t('文件来源')" class="file-source-card">
      <div class="label">{{ $t('选择文件') }}</div>
      <div class="upload-wrap">
        <bk-upload
          class="file-uploader"
          theme="button"
          accept=".xlsx, .xls, .csv, .sql"
          :size="100000"
          :multiple="false"
          :custom-request="handleFileUpload">
          <template #trigger>
            <bk-button class="upload-button">
              <Upload fill="#979BA5" class="icon" />
              <span class="text">{{ $t('上传文件') }}</span>
            </bk-button>
          </template>
        </bk-upload>
        <div class="tips">
          {{ $t('支持 .xlsx / .xls / .csv / .sql 文件，后台会自动检测文件类型，配置项格式请参照') }}
          <span class="sample-text">{{ $t('示例文件包') }}</span>
        </div>
      </div>
      <div v-if="uploadFile" class="file-wrapper">
        <div class="status-icon-area">
          <Done v-if="uploadFile.status === 'success'" class="success-icon" />
          <Error v-if="uploadFile.status === 'fail'" class="error-icon" />
        </div>
        <ExcelFill class="file-icon" />
        <div class="file-content">
          <div class="name">{{ uploadFile.name }}</div>
          <div v-if="uploadFile.status === 'uploading'" class="progress">
            <bk-progress :percent="uploadFile.progress" :theme="'primary'" size="small" />
          </div>
        </div>
      </div>
      <div v-if="uploadFile && uploadFile.status === 'success'" class="sheet">
        <div class="label">{{ $t('工作表') }}</div>
        <bk-select
          :model-value="selectSheet?.table_name"
          class="sheet-select"
          :clearable="false"
          :filterable="false"
          @change="handleSelectSheet">
          <bk-option v-for="item in sheetList" :id="item.table_name" :key="item.table_name" :name="item.table_name" />
        </bk-select>
      </div>
    </Card>
    <Card :title="$t('字段设置')">
      <FieldsTable
        v-if="selectSheet.table_name"
        ref="tableRef"
        :list="selectSheet!.columns as IFieldsItemEditing[]"
        :is-import="false"
        @change="handleFieldsChange" />
      <bk-exception
        v-else
        class="exception-wrap-item"
        :description="$t('请先上传文件')"
        :title="$t('暂无数据')"
        type="empty" />
    </Card>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { Upload, ExcelFill, Done, Error } from 'bkui-vue/lib/icon';
  import { importTable } from '../../../../../api/kv-table';
  import { ILocalTableImportItem, IFieldsItemEditing } from '../../../../../../types/kv-table';
  import FieldsTable from './../components/fields-table/upload.vue';
  import Card from '../../component/card.vue';

  interface IUploadFile {
    name: string;
    status: string;
    progress: number;
  }
  const props = defineProps<{
    bkBizId: string;
  }>();
  const emits = defineEmits(['change']);

  const uploadFile = ref<IUploadFile>();
  const tableRef = ref();
  const sheetList = ref<ILocalTableImportItem[]>([]);
  const selectSheet = ref<ILocalTableImportItem>({
    table_name: '',
    rows: [],
    columns: [],
  });

  const handleFileUpload = async (option: { file: File }) => {
    try {
      sheetList.value = [];
      selectSheet.value = {
        table_name: '',
        rows: [],
        columns: [],
      };
      uploadFile.value = {
        name: option.file.name,
        status: 'uploading',
        progress: 0,
      };
      sheetList.value = await importTable(
        props.bkBizId,
        0,
        option.file.name.split('.').pop() as string,
        option.file,
        (progress: number) => {
          uploadFile.value!.progress = progress;
        },
      );
      translateFileds();
      uploadFile.value!.status = 'success';
      selectSheet.value = sheetList.value[0];
      handleChange();
    } catch (error) {
      console.error(error);
      uploadFile.value!.status = 'fail';
    }
  };

  const handleSelectSheet = (sheet: string) => {
    selectSheet.value = sheetList.value.find((item) => item.table_name === sheet) as ILocalTableImportItem;
    handleChange();
  };

  // 接口数据转表单数据
  const translateFileds = () => {
    sheetList.value.forEach((sheet: ILocalTableImportItem) => {
      sheet.columns = sheet.columns.map((item: any, index: number) => {
        if (index === 0) {
          item.primary = true;
          item.unique = true;
          item.not_null = true;
        }
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
    });
  };

  const handleFieldsChange = (val: IFieldsItemEditing[]) => {
    selectSheet.value.columns = val;
    handleChange();
  };

  // 表单数据转接口数据
  const handleChange = () => {
    const columns = selectSheet.value.columns.map((item: any) => {
      let default_value;
      if (item.column_type === 'enum' && item.selected && item.default_value) {
        default_value = JSON.stringify(item.default_value);
      } else {
        default_value = String(item.default_value);
        if (item.default_value === null) {
          default_value = '';
        }
      }
      let enum_value;
      if (item.column_type === 'enum' && Array.isArray(item.enum_value)) {
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
    emits('change', columns, selectSheet.value.rows);
  };

  defineExpose({
    validate: async () => {
      return tableRef.value.validate();
    },
  });
</script>

<style scoped lang="scss">
  .file-source-card {
    margin-bottom: 16px;
    .label {
      position: relative;
      font-size: 12px;
      color: #63656e;
      &::after {
        position: absolute;
        top: 0;
        width: 14px;
        color: #ea3636;
        text-align: center;
        content: '*';
      }
    }
    .file-uploader {
      margin-top: 6px;
      :deep(.bk-upload-list) {
        display: none;
      }
    }
    .upload-button {
      .icon {
        font-size: 16px;
      }
      .text {
        font-size: 12px;
        margin-left: 7px;
        color: #63656e;
      }
    }
    .upload-wrap {
      display: flex;
      align-items: center;
      .tips {
        margin-left: 12px;
        color: #979ba5;
        letter-spacing: 0;
        font-size: 12px;
        .sample-text {
          color: #3a84ff;
          margin-left: 4px;
          cursor: pointer;
        }
      }
    }
    .file-wrapper {
      display: flex;
      align-items: center;
      color: #979ba5;
      font-size: 12px;
      height: 32px;
      margin-top: 8px;
      .status-icon-area {
        display: flex;
        width: 20px;
        height: 100%;
        align-items: center;
        justify-content: center;
        margin-right: 8px;
        .success-icon {
          font-size: 20px;
          color: #2dcb56;
        }
        .error-icon {
          font-size: 14px;
          color: #ea3636;
        }
      }
      .file-icon {
        margin: 0 6px 0 0;
        font-size: 14px;
      }
      .file-content {
        position: relative;
        width: 100%;
        height: 20px;
        .name {
          max-width: 360px;
          color: #63656e;
          white-space: nowrap;
          text-overflow: ellipsis;
          overflow: hidden;
        }
        :deep(.bk-progress) {
          position: absolute;
          width: 300px;
          bottom: -6px;
          .progress-outer {
            position: relative;
            .progress-text {
              position: absolute;
              right: 8px;
              top: -22px;
              font-size: 12px !important;
              color: #63656e !important;
            }
            .progress-bar {
              height: 2px;
            }
          }
        }
      }
    }
    .sheet {
      margin-top: 24px;
      .sheet-select {
        margin-top: 6px;
        width: 428px;
      }
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
