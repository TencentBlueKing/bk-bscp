<template>
  <div ref="tableRef" class="table-wrap">
    <vxe-table
      :data="data"
      :max-height="tableMaxHeight"
      show-footer-overflow
      show-overflow="tooltip"
      :loading="props.loading">
      <vxe-column :title="$t('服务别名')" width="170">
        <template #default="{ row }">
          <bk-button size="small" text theme="primary" @click="handleJump(row.id, 'service-config')">
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
            {{ getIsFileType(row) ? $t('文件型') : $t('table_键值型') }}
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
          <bk-tag
            :theme="row.spec.is_approve ? 'success' : ''"
            v-bk-tooltips="{ content: getApproveContent(row), disabled: !row.spec.is_approve, placement: 'right' }">
            {{ row.spec.is_approve ? $t('启用') : $t('关闭') }}
          </bk-tag>
        </template>
      </vxe-column>
      <vxe-column field="revision.creator" :title="$t('创建人')" width="123" />
      <vxe-column field="revision.reviser" :title="$t('更新人')" width="123" />
      <vxe-column :title="$t('更新时间')" width="200">
        <template #default="{ row }">
          <span>{{ datetimeFormat(row.revision.update_at) }}</span>
        </template>
      </vxe-column>
      <vxe-column :title="$t('操作')" :width="locale === 'zh-cn' ? 200 : 260">
        <template #default="{ row }">
          <div class="operation-wrap">
            <bk-button size="small" text theme="primary" @click="handleJump(row.id, 'service-config')">
              {{ $t('配置管理') }}
            </bk-button>
            <bk-button size="small" text theme="primary" @click="handleJump(row.id, 'client-search')">
              {{ $t('客户端查询') }}
            </bk-button>
            <MoreAction
              :app="row"
              :space-id="props.spaceId"
              @edit="emits('edit', row)"
              @delete="emits('delete', row)" />
          </div>
        </template>
      </vxe-column>
      <template #empty>
        <slot name="empty"></slot>
      </template>
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
  import { useRouter } from 'vue-router';
  import { IAppItem } from '../../../../../../types/app';
  import { useI18n } from 'vue-i18n';
  import { datetimeFormat } from '../../../../../utils';
  import { IPagination } from '../../../../../../types/index';
  import MoreAction from './more-action.vue';

  const { t, locale } = useI18n();
  const router = useRouter();

  const props = defineProps<{
    spaceId: string;
    data: IAppItem[];
    pagination: IPagination;
    loading: boolean;
  }>();

  const emits = defineEmits(['pageChange', 'limitChange', 'edit', 'delete']);

  const tableRef = ref();

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

  const getApproveContent = (row: IAppItem) => {
    const type = row.spec.approve_type === 'or_sign' ? t('或签') : t('会签');
    return `${t('审批人')}: ${row.spec.approver}\n${t('审批方式')}: ${type}`;
  };

  const handleJump = (id: number, name: string) => {
    if (name === 'service-config') {
      router.push({ name, params: { spaceId: props.spaceId, appId: id } });
    } else {
      const routeData = router.resolve({
        name,
        params: { spaceId: props.spaceId, appId: id },
      });
      window.open(routeData.href, '_blank');
    }
  };
</script>

<style scoped lang="scss">
  .table-wrap {
    height: 100%;
    width: 100%;
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
