<template>
  <table class="data-table">
    <thead>
      <tr>
        <th v-for="(item, index) in fieldsList" :key="index">
          <div class="fields-cell">
            <div class="fields-content">
              <div class="show-name">{{ item.name }}</div>
              <div class="fields-name">{{ item.alias }}</div>
            </div>
            <BatchSetPop
              v-if="!item.primary"
              :type="item.column_type"
              :is-multiple="item.selected"
              :enum-value="item.enum_value"
              @confirm="handleConfirmBatchSet" />
          </div>
        </th>
        <th class="operation">{{ $t('操作') }}</th>
      </tr>
    </thead>
    <tbody class="table-body">
      <tr v-for="(tableItem, index) in tableData" :key="index">
        <td v-for="field in fieldsList" :key="field.name">
          <template v-if="tableItem.content[field.name]">
            {{ tableItem.content[field.name] }}
          </template>
        </td>
        <td class="operation">
          <div class="action-btns">
            <i class="bk-bscp-icon icon-add" @click="handleAddData(index)"></i>
            <i class="bk-bscp-icon icon-reduce"></i>
          </div>
        </td>
      </tr>
    </tbody>
  </table>
</template>

<script lang="ts" setup>
  import { watch, ref } from 'vue';
  import { IFiledsItemEditing, IFiledItem, ILocalTableEditData } from '../../../../../../types/kv-table';
  import BatchSetPop from './batch-set-pop.vue';

  const props = defineProps<{
    fields: IFiledItem[];
    data: any[];
  }>();

  const fieldsList = ref<IFiledsItemEditing[]>([]);
  const tableData = ref<ILocalTableEditData[]>([]);

  watch(
    () => props.fields,
    () => {
      fieldsList.value = props.fields.map((item) => {
        let default_value: any;
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
            default_value = item.default_value;
          }
        } else {
          enum_value = item.enum_value;
        }
        return {
          ...item,
          enum_value,
          default_value,
          id: Date.now() + item.name,
          isShowBatchSet: false,
        };
      });
    },
  );

  watch(
    () => props.data,
    () => {
      // fieldsList.value.forEach((item) => {});
      if (props.data.length) {
        tableData.value = props.data.map((item) => {
          return {
            id: item.id,
            ...item.spec,
          };
        });
      } else {
        handleAddData(0);
      }
    },
  );

  const handleAddData = (index: number) => {
    const content: { [key: string]: string | string[] } = {};
    fieldsList.value.forEach((item) => {
      content[item.name] = item.default_value;
    });
    tableData.value.splice(index + 1, 0, { id: Date.now(), content, status: 'ADD' });
  };

  const batchSetStr = ref('');

  const handleConfirmBatchSet = () => {
    console.log(batchSetStr.value);
  };
</script>

<style scoped lang="scss">
  .data-table {
    width: 100%;
    border-collapse: collapse;
    th,
    td {
      border: 1px solid #dddddd;
      text-align: left;
      padding: 8px 16px;
      height: 42px;
    }
    thead {
      background: #f0f1f5;
      .fields-cell {
        display: flex;
        align-items: center;
        justify-content: space-between;
        .fields-content {
          font-size: 12px;
          line-height: 20px;
          .show-name {
            color: #313238;
          }
          .fields-name {
            color: #979ba5;
          }
        }
        .edit-line {
          color: #3a84ff;
        }
      }
      .operation {
        width: 120px;
      }
    }
    tbody {
      .operation {
        .action-btns {
          display: flex;
          align-items: center;
          justify-content: space-between;
          width: 38px;
          font-size: 14px;
          color: #979ba5;
          cursor: pointer;
          i:hover {
            color: #3a84ff;
          }
        }
      }
    }
  }

  .pop-wrap {
    width: 272px;
    .pop-content {
      padding: 16px;
      .pop-title {
        line-height: 24px;
        font-size: 16px;
        padding-bottom: 10px;
      }
    }

    .pop-footer {
      position: relative;
      height: 42px;
      background: #fafbfd;
      border-top: 1px solid #dcdee5;
      .button {
        position: absolute;
        right: 16px;
        top: 50%;
        transform: translateY(-50%);
      }
    }
  }
</style>

<style>
  .popover-wrap {
    padding: 0 !important;
  }
</style>
