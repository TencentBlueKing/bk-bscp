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
            <bk-popover
              v-if="!item.primary"
              ext-cls="popover-wrap"
              theme="light"
              trigger="manual"
              placement="bottom"
              :is-show="item.isShowBatchSet">
              <EditLine
                class="edit-line"
                v-bk-tooltips="{ content: $t('批量设置字段值') }"
                @click="item.isShowBatchSet = true" />
              <template #content>
                <div class="pop-wrap" v-click-outside="() => (item.isShowBatchSet = false)">
                  <div class="pop-content">
                    <div class="pop-title">{{ $t('批量设置字段值') }}</div>
                    <bk-input v-model="batchSetStr"></bk-input>
                  </div>
                  <div class="pop-footer">
                    <div class="button">
                      <bk-button
                        theme="primary"
                        style="margin-right: 8px"
                        size="small"
                        @click="handleConfirmBatchSet()">
                        {{ $t('确定') }}
                      </bk-button>
                      <bk-button size="small" @click="item.isShowBatchSet = false">{{ $t('取消') }}</bk-button>
                    </div>
                  </div>
                </div>
              </template>
            </bk-popover>
          </div>
        </th>
        <th class="operation">{{ $t('操作') }}</th>
      </tr>
    </thead>
    <tbody class="table-body">
      <tr v-for="tableItem in tableData" :key="tableItem.name">
        <td v-for="(field, index) in fieldsList" :key="index">
          <template v-if="tableItem.content[field.name]">
            {{ tableItem.content[field.name] }}
          </template>
        </td>
        <td class="operation">
          <div class="action-btns">
            <i class="bk-bscp-icon icon-add"></i>
            <i class="bk-bscp-icon icon-reduce"></i>
          </div>
        </td>
      </tr>
    </tbody>
  </table>
</template>

<script lang="ts" setup>
  import { watch, ref } from 'vue';
  import { EditLine } from 'bkui-vue/lib/icon';
  import { ITableFiledItem, IFiledsItem } from '../../../../../../types/kv-table';

  const props = defineProps<{
    fields: ITableFiledItem[];
    data: any[];
  }>();

  const fieldsList = ref<IFiledsItem[]>([]);
  const tableData = ref();

  watch(
    () => props.fields,
    () => {
      fieldsList.value = props.fields.map((item) => {
        return {
          ...item,
          isShowBatchSet: false,
        };
      });
    },
  );

  watch(
    () => props.data,
    () => {
      // fieldsList.value.forEach((item) => {});
      tableData.value = props.data.map((item) => {
        return item.spec;
      });
    },
  );

  // const tableData = ref([
  //   {
  //     showName: '唯一id',
  //     name: 'Id',
  //     isShowBatchSet: false,
  //     list: [1, 2, 3, 4, 5],
  //     isPrimaryKey: true,
  //   },
  //   {
  //     showName: '姓名',
  //     name: 'name',
  //     isShowBatchSet: false,
  //     list: [1, 2, 3, 4, 5],
  //     isPrimaryKey: false,
  //   },
  //   {
  //     showName: '年龄',
  //     name: 'age',
  //     isShowBatchSet: false,
  //     list: [1, 2, 3, 4, 5],
  //     isPrimaryKey: false,
  //   },
  //   {
  //     showName: '性别',
  //     name: 'gender',
  //     isShowBatchSet: false,
  //     list: [1, 2, 3, 4, 5],
  //     isPrimaryKey: false,
  //   },
  //   {
  //     showName: '唯一id',
  //     name: 'Id',
  //     isShowBatchSet: false,
  //     list: [1, 2, 3, 4, 5],
  //     isPrimaryKey: false,
  //   },
  // ]);

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
