<template>
  <DetailLayout :name="$t('新建表格')" :show-footer="!!selectedType" @close="handleCloseCreate">
    <template #content>
      <div class="create-table-content">
        <Card :title="$t('表结构')" class="table-source-card">
          <div class="table-source-type">
            <div
              v-for="item in tableStructureSource"
              :key="item.value"
              :class="['table-source-type-item', { active: selectedType === item.value }]"
              @click="selectedType = item.value">
              <div class="header">
                <i class="bk-bscp-icon icon-revoke" />
                <span class="label">{{ item.label }}</span>
              </div>
              <div class="info">{{ item.info }}</div>
            </div>
          </div>
        </Card>
        <ManualCreate
          v-if="selectedType === 'create'"
          ref="formRef"
          :is-manual-create="true"
          :bk-biz-id="spaceId"
          :is-edit="false" />
        <ImportFormLocal v-else-if="selectedType === 'import'" ref="formRef" :bk-biz-id="spaceId" />
      </div>
    </template>
    <template #footer>
      <div class="operation-btns">
        <bk-button theme="primary" style="width: 88px" @click="handleCreate">{{ $t('创建') }}</bk-button>
        <bk-button style="width: 130px">{{ $t('创建并编辑数据') }}</bk-button>
        <bk-button style="width: 88px" @click="handleCloseCreate">{{ $t('取消') }}</bk-button>
      </div>
    </template>
  </DetailLayout>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { storeToRefs } from 'pinia';
  import useGlobalStore from '../../../../../store/global';
  import DetailLayout from '../../component/detail-layout.vue';
  import Card from '../../component/card.vue';
  import ManualCreate from '../components/table-structure-form.vue';
  import ImportFormLocal from './import-form-local/index.vue';
  import { useI18n } from 'vue-i18n';

  const { t } = useI18n();

  const emits = defineEmits(['close']);

  const { spaceId } = storeToRefs(useGlobalStore());

  const selectedType = ref('create');
  const formRef = ref();

  const tableStructureSource = [
    {
      label: t('手动创建表结构'),
      value: 'create',
      info: t('目前没有表格结构及数据信息，需要先手动创建表结构，然后手动录入数据'),
    },
    {
      label: t('从本地文件导入'),
      value: 'import',
      info: t(
        '可以从本地导入 Excel/CSV 格式的数据文件（.xlsx/.xls/.csv)，还可以从带有 .sql 后缀的 MySQL dump 文件中导入表结构与数据',
      ),
    },
  ];

  const handleCreate = async () => {
    formRef.value.create();
    emits('close');
  };

  const handleCloseCreate = () => {
    emits('close');
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
        .bk-bscp-icon {
          font-size: 24px;
          margin-right: 7px;
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
