<template>
  <div class="table-structure-wrap">
    <section class="fields-setting">
      <div class="title">{{ $t('字段设置') }}</div>
      <ViewTable :fileds-list="fieldsList" />
    </section>
    <section class="basic-info">
      <div class="title">{{ $t('基本信息') }}</div>
      <div class="info-content">
        <div class="info-item">
          <div class="info-title">{{ $t('表格名称') }}</div>
          <div class="content">{{ 'aaaa' }}</div>
        </div>
        <div class="info-item">
          <div class="info-title">{{ $t('表格描述') }}</div>
          <div class="content">{{ '--' }}</div>
        </div>
      </div>
    </section>
    <section class="visible-range">
      <div class="title">{{ $t('可见范围') }}</div>
      <div class="info-item">
        <div class="info-title">{{ $t('服务') }}</div>
        <div class="content">{{ '全部服务' }}</div>
      </div>
    </section>
  </div>
</template>

<script lang="ts" setup>
  import ViewTable from './view-table.vue';

  const fieldsList = [
    {
      fieldsName: '123',
      showName: '123',
      type: 'enum',
      required: false,
      default: '',
      primaryKey: true,
      nonEmpty: false,
      only: false,
      autoIncrement: false,
      readonly: false,
      isShowSettingEnumPopover: false,
      enumList: [
        {
          text: '男',
          value: 'man',
        },
        {
          text: '女',
          value: 'woman',
        },
      ],
    },
  ];
</script>

<style scoped lang="scss">
  .table-structure-wrap {
    display: flex;
    flex-direction: column;
    gap: 32px;
    padding: 16px 24px;
    height: calc(100% - 42px);
    overflow: auto;
    background-color: #fff;
    .title {
      font-weight: 700;
      font-size: 14px;
      color: #63656e;
      margin-bottom: 16px;
    }
    .info-content {
      display: flex;
    }
    .info-item {
      width: 200px;
      font-size: 12px;
      .info-title {
        color: #979ba5;
        margin-bottom: 4px;
      }
      .content {
        color: #313238;
      }
    }
  }
</style>
