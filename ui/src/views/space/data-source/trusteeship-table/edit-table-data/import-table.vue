<template>
  <bk-dialog
    :is-show="props.show"
    :title="$t('导入')"
    width="960"
    height="720"
    ext-cls="import-table-dialog"
    :esc-close="false"
    :before-close="handleBeforeClose"
    @closed="emits('update:show', false)">
    <div class="select-file">
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
              <Upload fill="#979BA5" />
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
          <div v-if="uploadFile.status !== 'success'" class="progress">
            <bk-progress
              :percent="uploadFile.progress"
              :theme="uploadFile.status === 'fail' ? 'danger' : 'primary'"
              size="small" />
          </div>
        </div>
      </div>
    </div>
    <div v-if="uploadFile && uploadFile.status === 'success'" class="sheet">
      <div class="label">{{ $t('工作表') }}</div>
      <div class="sheet-content">
        <bk-select
          :model-value="selectSheet?.table_name"
          class="sheet-select"
          :clearable="false"
          :filterable="false"
          @change="handleSelectSheet">
          <bk-option v-for="item in sheetList" :id="item.table_name" :key="item.table_name" :name="item.table_name" />
        </bk-select>
        <div class="sheet-status">
          <div v-if="sheetStatus === 'loading'" class="status-content">
            <Spinner class="spinner-icon icon" />
            <span>{{ $t('正在匹配表格字段') }}</span>
          </div>
          <div v-else-if="sheetStatus === 'success'" class="status-content">
            <Success class="success-icon icon" />
            <span>{{ $t('表格字段匹配，可继续导入') }}</span>
          </div>
          <div v-else class="status-content">
            <Warn class="warn-icon icon" />
            <span>{{ $t('表格字段有差异，请先确认调整') }}</span>
          </div>
        </div>
      </div>
      <bk-checkbox v-model="isClearData" class="clear-data"> {{ $t('导入前清空原有数据') }} </bk-checkbox>
    </div>
    <div v-if="selectSheet.table_name" class="fields-setting">
      <div class="header">
        <span class="title">{{ $t('字段设置') }}</span>
        <span class="info">
          <InfoLine class="info-icon" />
          <span class="label">{{ $t('主键校验：') }}</span>
          <span class="content">{{ $t('当导入表的主键值出现重合，将直接') }}</span>
          <span class="warn">{{ $t('使用导入表的数据覆盖') }}</span>
          <span>。</span>
          <span class="label">{{ $t('字段校验：') }}</span>
          <span class="content">{{ $t('导入表删除字段后将更新表结构，该字段') }}</span>
          <span class="warn">{{ $t('已有数据也将同时被清空') }}</span>
          <span>。</span>
        </span>
      </div>
      <div class="fields">
        <UploadFieldsTable :list="selectSheet.columns" :is-import="true" @change="handleChangeFields" />
      </div>
    </div>
    <template #footer>
      <bk-button theme="primary" style="margin-right: 8px" @click="handleImport">
        {{ $t('导入') }}
      </bk-button>
      <bk-button @click="emits('update:show', false)">{{ $t('取消') }}</bk-button>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { Upload, ExcelFill, Done, Error, Success, Warn, Spinner, InfoLine } from 'bkui-vue/lib/icon';
  import UploadFieldsTable from '../components/fields-table/upload.vue';
  import { ILocalTableImportItem, IFieldsItemEditing } from '../../../../../../types/kv-table';
  import { importTable } from '../../../../../api/kv-table';

  const props = defineProps<{
    show: boolean;
    bkBizId: string;
    id: number;
  }>();

  const emits = defineEmits(['update:show']);
  const sheetStatus = ref('warn');
  const isClearData = ref(false);
  const sheetList = ref<ILocalTableImportItem[]>([]);
  const selectSheet = ref<ILocalTableImportItem>({
    table_name: '',
    rows: [],
    columns: [],
  });

  const uploadFile = ref();

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
        props.id,
        option.file.name.split('.').pop() as string,
        option.file,
        (progress: number) => {
          uploadFile.value!.progress = progress;
        },
      );
      translateFileds();
      uploadFile.value!.status = 'success';
      selectSheet.value = sheetList.value[0];
      // handleChange();
    } catch (error) {
      console.error(error);
      uploadFile.value!.status = 'fail';
    }
  };

  // 接口数据转表单数据
  const translateFileds = () => {
    sheetList.value.forEach((sheet: ILocalTableImportItem) => {
      sheet.columns = sheet.columns.map((item: any, index: number) => {
        if (index === 0) {
          item.primary = true;
          item.unique = true;
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

  const handleSelectSheet = (sheet: string) => {
    selectSheet.value = sheetList.value.find((item) => item.table_name === sheet) as ILocalTableImportItem;
  };

  const handleChangeFields = (val: IFieldsItemEditing[]) => {
    selectSheet.value.columns = val;
  };

  // 表单数据转接口数据
  // const handleChange = () => {
  //   const columns = selectSheet.value.columns.map((item: any) => {
  //     let default_value;
  //     if (item.column_type === 'enum' && item.selected && item.default_value) {
  //       default_value = JSON.stringify(item.default_value);
  //     } else {
  //       default_value = String(item.default_value);
  //       if (item.default_value === null) {
  //         default_value = '';
  //       }
  //     }
  //     let enum_value;
  //     if (item.column_type === 'enum' && Array.isArray(item.enum_value)) {
  //       enum_value = JSON.stringify(item.enum_value);
  //     } else {
  //       enum_value = '';
  //     }
  //     return {
  //       default_value,
  //       enum_value, // 枚举值设置内容
  //       name: item.name,
  //       alias: item.alias,
  //       primary: item.primary,
  //       column_type: item.column_type,
  //       not_null: item.not_null,
  //       unique: item.unique,
  //       read_only: item.read_only,
  //       auto_increment: item.auto_increment,
  //       selected: item.selected,
  //     };
  //   });
  // };

  const handleBeforeClose = () => {};

  const handleImport = () => {};
</script>

<style scoped lang="scss">
  .select-file {
    font-size: 12px;
    line-height: 20px;
    .label {
      color: #63656e;
      margin-bottom: 6px;
      &::after {
        display: inline-block;
        content: '*';
        color: #ea3636;
        margin-left: 4px;
      }
    }
    .upload-wrap {
      display: flex;
      align-items: center;
      .tips {
        margin-left: 12px;
        color: #979ba5;
        letter-spacing: 0;
        .sample-text {
          color: #3a84ff;
          margin-left: 4px;
          cursor: pointer;
        }
      }
    }
    .file-uploader {
      margin-top: 6px;
      :deep(.bk-upload-list) {
        display: none;
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
  }
  .sheet {
    @extend .select-file;
    margin: 24px 0;
    .sheet-content {
      display: flex;
      align-items: center;
      .sheet-select {
        width: 428px;
        margin-right: 8px;
      }
      .sheet-status {
        color: #63656e;
        .status-content {
          display: flex;
          align-items: center;
        }
        .icon {
          font-size: 14px;
          margin-right: 5px;
        }
        .spinner-icon {
          color: #3a84ff;
        }
        .success-icon {
          color: #2dcb56;
        }
        .warn-icon {
          color: #ff9c01;
        }
      }
    }
    .bk-checkbox {
      margin-top: 21px;
    }
  }

  .fields-setting {
    margin-top: 24px;
    border-top: 1px solid #dcdee5;
    .header {
      display: flex;
      align-items: center;
      margin: 10px 0 16px;
      .title {
        font-weight: 700;
        font-size: 14px;
        color: #63656e;
        line-height: 22px;
        margin-right: 16px;
      }
      .info {
        display: flex;
        align-items: center;
        font-size: 12px;
        .info-icon {
          font-size: 14px;
          color: #979ba5;
          margin-right: 5px;
        }
        .label {
          color: #63656e;
        }
        .content {
          color: #979ba5;
        }
        .warn {
          color: #ff9c01;
        }
      }
    }
  }
  .fields {
    max-height: 280px;
    overflow: auto;
  }
</style>

<style lang="scss">
  .import-table-dialog {
    .bk-modal-content {
      height: calc(100% - 50px) !important;
      overflow: auto;
    }
  }
</style>
