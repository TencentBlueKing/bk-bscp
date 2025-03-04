<template>
  <div ref="tableRef" class="table-wrap">
    <vxe-table :data="data" :max-height="tableMaxHeight" show-footer-overflow>
      <vxe-column :title="$t('服务别名')" width="170">
        <template #default="{ row }">
          <bk-button size="small" text theme="primary" @click="handleToManage(row)">
            {{ row.spec.alias }}
          </bk-button>
        </template>
      </vxe-column>
      <vxe-column field="spec.name" :title="$t('服务名称')" width="160" />
      <vxe-column :title="$t('服务描述')" min-width="160">
        <template #default="{ row }">
          <span>{{ row.spec.memo || '--' }}</span>
        </template>
      </vxe-column>
      <vxe-column :title="$t('配置类型')" width="110">
        <template #default="{ row }">
          <bk-tag :theme="getIsFileType(row) ? 'info' : 'warning'">
            {{ getIsFileType(row) ? $t('文件型') : $t('键值型') }}
          </bk-tag>
        </template>
      </vxe-column>
      <vxe-column :title="$t('配置格式限制')" width="110">
        <template #default="{ row }">
          <span>{{ getKvDataType(row) }}</span>
        </template>
      </vxe-column>
      <vxe-column :title="$t('上线审批')" width="100">
        <template #default="{ row }">
          <bk-tag :theme="row.spec.is_approve ? 'success' : ''">
            {{ row.spec.is_approve ? $t('启用') : $t('关闭') }}
          </bk-tag>
        </template>
      </vxe-column>
      <vxe-column field="revision.creator" :title="$t('创建人')" width="123" />
      <vxe-column field="revision.reviser" :title="$t('更新人')" width="123" />
      <vxe-column :title="$t('更新时间')" width="160">
        <template #default="{ row }">
          <span>{{ datetimeFormat(row.revision.update_at) }}</span>
        </template>
      </vxe-column>
      <vxe-column :title="$t('操作')" width="200">
        <template #default="{ row }">
          <div class="operation-wrap">
            <bk-button size="small" text theme="primary" @click="handleToManage(row)">
              {{ $t('配置管理') }}
            </bk-button>
            <bk-button size="small" text theme="primary" @click="handleToClient(row)">
              {{ $t('客户端查询') }}
            </bk-button>
            <bk-popover
              ref="popoverRef"
              theme="light"
              trigger="hover"
              placement="bottom"
              :arrow="false"
              :offset="{ mainAxis: 10, crossAxis: 30 }">
              <Ellipsis class="ellipsis" />
              <template #content>
                <ul class="dropdown-ul">
                  <li class="dropdown-li" v-for="item in operationList" :key="item.name">
                    {{ item.name }}
                  </li>
                </ul>
              </template>
            </bk-popover>
          </div>
        </template>
      </vxe-column>
    </vxe-table>
    <bk-pagination
      class="table-pagination"
      :model-value="props.pagination.current"
      :count="props.pagination.count"
      :limit="props.pagination.limit"
      location="left"
      :layout="['total', 'limit', 'list']"
      @change="emits('pageChange', $event)"
      @limit-change="emits('limitChange', $event)" />
  </div>
</template>

<script lang="ts" setup>
  import { ref, computed } from 'vue';
  import { IAppItem } from '../../../../../../types/app';
  import { useI18n } from 'vue-i18n';
  import { datetimeFormat } from '../../../../../utils';
  import { Ellipsis } from 'bkui-vue/lib/icon';
  import { IPagination } from '../../../../../../types/index';

  const { t } = useI18n();
  const props = defineProps<{
    data: IAppItem[];
    pagination: IPagination;
  }>();

  const emits = defineEmits(['pageChange', 'limitChange']);

  const tableRef = ref();
  const operationList = [
    {
      name: t('客户端统计'),
    },
    {
      name: t('编辑基本属性'),
    },
    {
      name: t('配置示例'),
    },
    {
      name: t('操作记录'),
    },
    {
      name: t('删除'),
    },
  ];

  const tableMaxHeight = computed(() => {
    return tableRef.value && tableRef.value.clientHeight - 60;
  });

  const getIsFileType = (row: IAppItem) => row.spec.config_type === 'file';

  const getKvDataType = (row: IAppItem) => {
    if (row.spec.data_type === 'any') {
      return t('任意类型');
    }
    if (row.spec.data_type === 'secret') {
      return t('敏感信息');
    }
    return row.spec.data_type || '--';
  };

  const handleToManage = (row: IAppItem) => {
    console.log(row);
  };

  const handleToClient = (row: IAppItem) => {
    console.log(row);
  };
</script>

<style scoped lang="scss">
  .table-wrap {
    height: 100%;
    .table-content {
      max-height: calc(100% - 60px);
      overflow: auto;
    }
  }
  .operation-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .ellipsis {
    font-size: 16px;
    transform: rotate(90deg);
    cursor: pointer;
  }

  .dropdown-ul {
    margin: -12px;
    font-size: 12px;
    .dropdown-li {
      padding: 0 12px;
      min-width: 68px;
      font-size: 12px;
      line-height: 32px;
      color: #4d4f56;
      cursor: pointer;
      &:hover {
        background: #f5f7fa;
      }
    }
  }
  .table-pagination {
    padding: 14px 16px;
    height: 60px;
    background: #fff;
    border: 1px solid #e8eaec;
    border-top: none;
    :deep(.bk-pagination-list.is-last) {
      margin-left: auto;
    }
  }
</style>
