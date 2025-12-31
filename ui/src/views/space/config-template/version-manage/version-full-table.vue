<template>
  <PrimaryTable class="version-table border" :data="props.list">
    <TableColumn :title="t('版本号')" prop="spec.revision_name">
      <template #default="{ row }">
        <div class="revision_name">
          <bk-button v-if="row.spec" text theme="primary" @click="emits('select', row.id)">
            {{ row.spec.revision_name }}
          </bk-button>
        </div>
      </template>
    </TableColumn>
    <TableColumn :title="t('版本描述')">
      <template #default="{ row }">
        <span v-if="row.spec">{{ row.spec.revision_memo || '--' }}</span>
      </template>
    </TableColumn>
    <TableColumn :title="t('更新人')">
      <template #default="{ row }">
        <user-name v-if="row.revision" :name="row.revision.rivision" />
      </template>
    </TableColumn>
    <TableColumn :title="t('更新时间')">
      <template #default="{ row }">
        <template v-if="row.revision">
          {{ datetimeFormat(row.revision.update_at) }}
        </template>
      </template>
    </TableColumn>
    <TableColumn :title="t('操作')" width="337">
      <template #default="{ row, rowIndex }">
        <div class="actions-wrapper">
          <bk-button v-if="rowIndex === 0" text theme="primary" @click="handleConfigIssue">
            {{ t('配置下发') }}
          </bk-button>
          <bk-button text theme="primary" @click="handleOpenDiffSlider(row)">{{ t('版本对比') }}</bk-button>
          <bk-button text theme="primary" @click="emits('create', row.id)">{{ t('复制并新建') }}</bk-button>
          <TableMoreActions :operation-list="operationList" @operation="handleOperation(row, $event)" />
        </div>
      </template>
    </TableColumn>
  </PrimaryTable>
  <bk-pagination
    class="table-pagination"
    :model-value="pagination.current"
    :count="pagination.count"
    :limit="pagination.limit"
    location="left"
    :layout="['total', 'limit', 'list']"
    @change="emits('page-value-change', $event)"
    @limit-change="emits('page-limit-change', $event)" />
  <TemplateVersionDiff
    v-model:show="diffSliderData.open"
    :space-id="spaceId"
    :template-space-id="templateSpaceId"
    :crt-version="diffSliderData.data" />
</template>
<script lang="ts" setup>
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import { IPagination } from '../../../../../types/index';
  import { ITemplateVersionItem, DiffSliderDataType } from '../../../../../types/template';
  import { datetimeFormat } from '../../../../utils/index';
  import { fileDownload } from '../../../../utils/file';
  import { downloadTemplateContent } from '../../../../api/template';
  import UserName from '../../../../components/user-name.vue';
  import TemplateVersionDiff from '../../templates/version-manage/template-version-diff.vue';
  import TableMoreActions from '../../../../components/table/table-more-actions.vue';

  const { t } = useI18n();
  const router = useRouter();

  const props = defineProps<{
    spaceId: string;
    templateSpaceId: number;
    templateId: number;
    configTemplateId: number;
    list: ITemplateVersionItem[];
    pagination: IPagination;
  }>();

  const emits = defineEmits([
    'page-value-change',
    'page-limit-change',
    'openVersionDiff',
    'select',
    'deleted',
    'create',
  ]);

  const diffSliderData = ref<{ open: boolean; data: DiffSliderDataType }>({
    open: false,
    data: { id: 0, versionId: 0, name: '' },
  });

  const operationList = [
    {
      name: t('下载'),
      id: 'download',
    },
  ];

  const handleOpenDiffSlider = (version: ITemplateVersionItem) => {
    const { id, spec } = version;
    diffSliderData.value = {
      open: true,
      data: { id: props.templateId, versionId: id, name: spec.revision_name, permission: spec.permission },
    };
  };

  const handleOperation = (version: ITemplateVersionItem, operation: string) => {
    if (operation === 'download') {
      handleDownload(version);
    }
  };

  const handleDownload = async (version: ITemplateVersionItem) => {
    const { name, revision_name, content_spec } = version.spec;
    const content = await downloadTemplateContent(props.spaceId, props.templateSpaceId, content_spec.signature, true);
    fileDownload(content, `${name}_${revision_name}`);
  };

  // 配置下发
  const handleConfigIssue = () => {
    router.push({
      name: 'config-issued',
      query: {
        templateIds: [props.configTemplateId],
      },
    });
  };
</script>
<style lang="scss" scoped>
  .version-table {
    width: 100%;
    background: #ffffff;
    .revision_name {
      display: flex;
      align-items: center;
      gap: 8px;
      height: 100%;
    }
  }
  .actions-wrapper {
    display: flex;
    align-items: center;
    gap: 8px;
    .more-actions {
      display: flex;
      align-items: center;
      justify-content: center;
      margin-left: 8px;
      width: 16px;
      height: 16px;
      border-radius: 50%;
      cursor: pointer;
      &:hover {
        background: #dcdee5;
        color: #3a84ff;
      }
      .ellipsis-icon {
        font-size: 16px;
        transform: rotate(90deg);
        cursor: pointer;
      }
    }
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
      &.disabled {
        color: #c4c6cc;
        cursor: not-allowed;
      }
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
  .delete-content {
    .label {
      color: #4d4f56;
      line-height: 22px;
    }
    .value {
      font-size: 14px;
      color: #313238;
      letter-spacing: 0;
      line-height: 22px;
    }
  }
</style>
