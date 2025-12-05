<template>
  <div class="table-wrap">
    <div class="head">
      <div class="head-left" @click="isShow = !isShow">
        <AngleDownFill :class="['angle-icon', isShow && 'expanded']" />
        <span class="template-name">{{ templateName }}</span>
        <span class="version">
          ({{ $t('即将下发 ') }} <span>{{ `#${list[0]?.latest_template_revision_name}` }}</span> {{ $t('版本') }})
        </span>
      </div>
      <div class="head-right">
        <bk-popover theme="light" trigger="click" placement="bottom">
          <span :class="['version-select-trigger', { checked: checkedVersion.length }]">
            <funnel class="funnel-icon" />
            <span v-if="checkedVersion.length">{{ $t('已选{n}/{m}个版本', { n: 1, m: 2 }) }}</span>
            <span v-else>{{ $t('按版本选择') }}</span>
          </span>
          <template #content>
            <div class="version-select-content">
              <div class="info">{{ $t('根据配置模板历史版本，筛选对应的实例') }}</div>
              <bk-checkbox-group v-model="checkedVersion" @change="handleSelectVersion">
                <bk-checkbox v-for="version in versions" :key="version.id" :label="version.name" />
              </bk-checkbox-group>
            </div>
          </template>
        </bk-popover>
        <span class="line"></span>
        <span>
          {{ $t('已选') }}
          <span class="count">{{ checkedVersion.length }}</span>
          {{ $t('个') }}
        </span>
      </div>
    </div>
    <div v-show="isShow">
      <PrimaryTable class="border" :data="list">
        <TableColumn :title="$t('进程别名')" col-key="process_alias"></TableColumn>
        <TableColumn :title="$t('所属拓扑')" ellipsis>
          <template #default="{ row }: { row: ITemplateProcessItem }">
            {{ `${row.set} / ${row.module} / ${row.service_instance}` }}
          </template>
        </TableColumn>
        <TableColumn col-key="cc_process_id">
          <template #title>
            <span class="tips-title" v-bk-tooltips="{ content: $t('对应 CMDB 中唯一 ID'), placement: 'top' }">
              {{ $t('CC 进程ID') }}
            </span>
          </template>
        </TableColumn>
        <TableColumn col-key="module_inst_seq">
          <template #title>
            <span class="tips-title" v-bk-tooltips="{ content: $t('模块下唯一标识'), placement: 'top' }"> InstID </span>
          </template>
        </TableColumn>
        <TableColumn :title="$t('版本ID')" col-key="config_version_name" />
        <TableColumn :title="$t('版本描述')" col-key="config_version_memo" />
        <TableColumn :title="$t('操作')">
          <template #default="{ row }: { row: ITemplateProcessItem }">
            <bk-button theme="primary" text @click="handleDiff(row)"> {{ $t('配置对比') }}</bk-button>
          </template>
        </TableColumn>
      </PrimaryTable>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, computed } from 'vue';
  import { ITemplateProcessItem } from '../../../../../../types/config-template';
  import { AngleDownFill, Funnel } from 'bkui-vue/lib/icon';

  const props = defineProps<{
    list: ITemplateProcessItem[];
    versions: { id: string; name: string }[];
  }>();
  const emits = defineEmits(['select']);

  const isShow = ref(true);
  const checkedVersion = ref<string[]>([]);

  const templateName = computed(() => {
    if (!props.list.length) return '';
    return `${props.list[0].config_template_name} / ${props.list[0].file_name}`;
  });

  const handleDiff = (row: ITemplateProcessItem) => {
    console.log(row);
  };

  const handleSelectVersion = (val: string[]) => {
    emits('select', val);
  };
</script>

<style scoped lang="scss">
  .table-wrap {
    .head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 16px;
      height: 42px;
      background: #dcdee5;
      cursor: pointer;
      .head-left {
        display: flex;
        align-items: center;
        gap: 8px;
        .angle-icon {
          margin-right: 4px;
          transition: transform 0.3s;
          color: #63656e;
          transform: rotate(90deg);
          &.expanded {
            transform: rotate(180deg);
            transition: transform 0.3s;
          }
        }
        .template-name {
          color: #63656e;
          font-weight: 700;
        }
        .version {
          font-size: 12px;
          color: #979ba5;
          font-weight: 700;
          span {
            color: #3a84ff;
          }
        }
      }
      .head-right {
        display: flex;
        align-items: center;
        font-size: 12px;
        .version-select-trigger {
          display: flex;
          align-items: center;
          gap: 4px;
          font-size: 12px;
          cursor: pointer;
          color: #4d4f56;
          .funnel-icon {
            color: #979ba5;
            font-size: 14px;
          }
          &.checked {
            color: #3a84ff;
            .funnel-icon {
              color: #3a84ff;
            }
          }
        }
        .line {
          width: 1px;
          height: 16px;
          background: #c4c6cc;
          margin: 0 12px;
        }
        .count {
          color: #3a84ff;
          font-weight: bold;
        }
      }
    }
  }
  .version-select-content {
    width: 260px;
    .info {
      margin-bottom: 12px;
    }
  }
</style>
