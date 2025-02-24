<template>
  <DetailLayout :name="$t('新建表格')" :show-footer="!!selectedType" @close="handleCloseCreate">
    <template #content>
      <div class="create-table-content">
        <Card :title="$t('表结构来源')" class="table-source-card">
          <div class="table-source-type">
            <div
              v-for="item in tableStructureSource"
              :key="item.value"
              :class="['table-source-type-item', { active: selectedType === item.value }]"
              @click="selectedType = item.value">
              <div class="header">
                <div class="svg-wrap">
                  <div :class="['svg', item.svg]"></div>
                </div>
                <span class="label">{{ item.label }}</span>
              </div>
              <div class="info">{{ item.info }}</div>
            </div>
          </div>
        </Card>
        <ManualCreate
          v-if="selectedType === 'create'"
          ref="fieldRef"
          :columns="fieldsColumns"
          :bk-biz-id="spaceId"
          @change="fieldsColumns = $event" />
        <ImportFormLocal
          v-else-if="selectedType === 'import'"
          ref="fieldRef"
          :bk-biz-id="spaceId"
          @change="handleUploadChange" />
        <baseInfoForm
          ref="formRef"
          :bk-biz-id="spaceId"
          :is-edit="false"
          :form="formData"
          @change="formData = $event" />
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary" style="width: 88px" :loading="loading" @click="handleCreate(false)">
          {{ $t('创建') }}
        </bk-button>
        <bk-button style="width: 130px" :loading="loading" @click="handleCreate(true)">
          {{ $t('创建并编辑数据') }}
        </bk-button>
        <bk-button style="width: 88px" @click="handleCloseCreate">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { storeToRefs } from 'pinia';
  import { IFieldItem, ILocalTableBase, ILocalTableEditQuery } from '../../../../../../types/kv-table';
  import { manualCreateTable, createStructAndContent } from '../../../../../api/kv-table';
  import { useRouter } from 'vue-router';
  import useGlobalStore from '../../../../../store/global';
  import DetailLayout from '../../component/detail-layout.vue';
  import Card from '../../component/card.vue';
  import ManualCreate from './manual-create.vue';
  import ImportFormLocal from './import-from-local.vue';
  import { useI18n } from 'vue-i18n';
  import BkMessage from 'bkui-vue/lib/message';
  import baseInfoForm from '../components/base-info-form.vue';

  const { t } = useI18n();

  const router = useRouter();

  const { spaceId } = storeToRefs(useGlobalStore());

  const selectedType = ref('create');
  const loading = ref(false);
  const fieldRef = ref();
  const formRef = ref();

  const formData = ref<ILocalTableBase>({
    table_name: '',
    table_memo: '',
    visible_range: ['*'],
  });

  const fieldsColumns = ref<IFieldItem[]>([]);
  const uploadTableData = ref<ILocalTableEditQuery[]>([]);

  const tableStructureSource = [
    {
      label: t('手动创建表结构'),
      value: 'create',
      info: t('目前没有表格结构及数据信息，需要先手动创建表结构，然后手动录入数据'),
      svg: 'icon-manual-create',
    },
    {
      label: t('从本地文件导入'),
      value: 'import',
      info: t(
        '可以从本地导入 Excel/CSV 格式的数据文件（.xlsx/.xls/.csv)，还可以从带有 .sql 后缀的 MySQL dump 文件中导入表结构与数据',
      ),
      svg: 'icon-import-local',
    },
  ];

  const handleUploadChange = (columns: IFieldItem[], data: ILocalTableEditQuery[]) => {
    fieldsColumns.value = columns;
    uploadTableData.value = data;
  };

  const handleCreate = async (redirectToEdit = false) => {
    try {
      const validate = (await formRef.value.validate()) && (await fieldRef.value.validate());
      if (!validate) return;
      loading.value = true;
      let res;
      if (selectedType.value === 'create') {
        // 手动创建表结构
        const data = {
          spec: {
            ...formData.value,
            columns: fieldsColumns.value,
          },
        };

        res = await manualCreateTable(spaceId.value, data);
      } else {
        const data = {
          spec: {
            ...formData.value,
            columns: fieldsColumns.value,
          },
          contents: uploadTableData.value,
        };

        res = await createStructAndContent(spaceId.value, data);
      }

      if (redirectToEdit) {
        // 跳转到编辑页面
        router.push({
          name: 'edit-table-data',
          params: { spaceId: spaceId.value, id: res.data.id },
          query: { name: formData.value.table_name },
        });
      } else {
        // 关闭创建弹窗
        handleCloseCreate();
      }
      BkMessage({ theme: 'success', message: t('新建表格成功') });
    } catch (error) {
      console.error(error);
    } finally {
      loading.value = false;
    }
  };

  const handleCloseCreate = () => {
    router.push({
      name: 'trusteeship-table-list',
      params: { spaceId: spaceId.value },
    });
  };
</script>

<style scoped lang="scss">
  .create-table-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    background: #f5f7fa;
    padding: 24px 0;
    min-height: 100%;
  }
  .table-source-type {
    display: flex;
    justify-content: space-between;
    .table-source-type-item {
      padding: 16px;
      width: 464px;
      height: 100px;
      border-radius: 2px;
      border: 1px solid #dcdee5;
      cursor: pointer;
      .header {
        display: flex;
        align-items: center;
        .svg-wrap {
          display: flex;
          align-items: center;
          margin-right: 4px;
          justify-content: center;
          width: 30px;
          height: 30px;
          background: #eaebf0;
          border-radius: 50%;
        }
        .svg {
          width: 20px;
          height: 20px;
          &.icon-manual-create {
            background: url('../../../../../assets/add-doc.svg') no-repeat;
          }
          &.icon-import-local {
            background: url('../../../../../assets/backup.svg') no-repeat;
          }
        }
        .label {
          font-size: 14px;
          color: #63656e;
        }
      }
      .info {
        margin-top: 12px;
        font-size: 12px;
        color: #979ba5;
      }
      &:hover {
        border: 1px solid #c4c6cc;
      }
      &.active {
        color: #3a84ff;
        border: 1px solid #3a84ff;
        background: #f0f5ff;
        cursor: pointer;
        .svg {
          &.icon-manual-create {
            background: url('../../../../../assets/add-doc-active.svg') no-repeat;
          }
          &.icon-import-local {
            background: url('../../../../../assets/backup-active.svg') no-repeat;
          }
        }
      }
    }
  }

  .table-source-card {
    :deep(.card-header) {
      margin-bottom: 12px;
    }
  }

  .operation-btns {
    height: 100%;
    width: 1000px;
    display: flex;
    gap: 8px;
    align-items: center;
  }
</style>
