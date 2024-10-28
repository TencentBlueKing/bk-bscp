<template>
  <bk-dialog
    :is-show="props.show"
    :title="$t('批量修改权限')"
    :theme="'primary'"
    quick-close
    ext-cls="batch-edit-perm-dialog"
    :width="640">
    <div class="selected-tag">
      {{ `${t('已选')} ` }} <span class="count">{{ props.configsLength }}</span> {{ `${t('个配置项')}` }}
    </div>
    <bk-form form-type="vertical" class="user-settings">
      <userSetting
        :bk-biz-id="props.bkBizId"
        :id="props.id"
        :form="localVal"
        :is-batch-edit="true"
        @change="localVal = $event" />
    </bk-form>
    <template v-if="currentPkg && currentPkg !== 'no_specified'">
      <p class="tips">{{ t('以下服务配置的未命名版本中引用此套餐的内容也将更新') }}</p>
      <div class="service-table">
        <bk-loading style="min-height: 100px" :loading="loading">
          <bk-table :data="citedList" :max-height="maxTableHeight">
            <bk-table-column :label="t('所在模板套餐')" prop="template_set_name"></bk-table-column>
            <bk-table-column :label="t('使用此套餐的服务')">
              <template #default="{ row }">
                <div v-if="row.app_id" class="app-info" @click="goToConfigPageImport(row.app_id)">
                  <div v-overflow-title class="name-text">{{ row.app_name }}</div>
                  <LinkToApp class="link-icon" :id="row.app_id" />
                </div>
              </template>
            </bk-table-column>
          </bk-table>
        </bk-loading>
      </div>
    </template>
    <template #footer>
      <bk-button
        theme="primary"
        style="margin-right: 8px"
        :loading="loading"
        :disabled="loading"
        @click="handleConfirm">
        {{ t('保存') }}
      </bk-button>
      <bk-button @click="emits('update:show', false)">{{ t('取消') }}</bk-button>
    </template>
  </bk-dialog>
</template>

<script lang="ts" setup>
  import { ref, computed, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { IPackagesCitedByApps, ITemplateConfigItem } from '../../../../../../../../types/template';
  import { useRouter } from 'vue-router';
  import { storeToRefs } from 'pinia';
  import { getUnNamedVersionAppsBoundByPackages, getPackagesByTemplateIds } from '../../../../../../../api/template';
  import useGlobalStore from '../../../../../../../store/global';
  import useTemplateStore from '../../../../../../../store/template';
  import LinkToApp from '../../../components/link-to-app.vue';
  import userSetting from '../../../../../service/detail/config/components/user-setting.vue';
  import { isEqual } from 'lodash';

  const { spaceId } = storeToRefs(useGlobalStore());
  const { currentTemplateSpace, currentPkg } = storeToRefs(useTemplateStore());
  const { t } = useI18n();
  const router = useRouter();

  const props = defineProps<{
    bkBizId: string;
    id: number;
    show: boolean;
    configsLength: number;
    loading: boolean;
    configs?: ITemplateConfigItem[];
  }>();

  const emits = defineEmits(['update:show', 'confirm']);

  const localVal = ref({
    privilege: '',
    user: '',
    user_group: '',
    uid: undefined,
    gid: undefined,
  });
  const citedList = ref<IPackagesCitedByApps[]>([]);
  const tableLoading = ref(false);
  const pkgsIds = ref<number[]>([]);
  const citeByAppIds = ref<number[]>([]);

  watch(
    () => props.show,
    (val) => {
      if (val) {
        localVal.value = {
          privilege: '',
          user: '',
          user_group: '',
          uid: undefined,
          gid: undefined,
        };
        if (currentPkg.value && currentPkg.value !== 'no_specified') {
          getCitedData();
        }
      }
    },
  );

  const maxTableHeight = computed(() => {
    const windowHeight = window.innerHeight;
    return windowHeight * 0.6 - 200;
  });

  // 配置项被套餐引用数据
  const loadCiteByPkgsCountList = async () => {
    const ids = props.configs!.map((item) => item.id);
    const res = await getPackagesByTemplateIds(spaceId.value, currentTemplateSpace.value, ids);
    res.details.forEach((item) =>
      item.forEach((template) => {
        if (pkgsIds.value?.includes(template.template_set_id)) return;
        pkgsIds.value?.push(template.template_set_id);
      }),
    );
  };

  const getCitedData = async () => {
    tableLoading.value = true;
    const params = {
      start: 0,
      all: true,
    };
    if (currentPkg.value === 'all') {
      await loadCiteByPkgsCountList();
    }
    const template_set_ids: number[] = currentPkg.value === 'all' ? pkgsIds.value : [currentPkg.value as number];
    const res = await getUnNamedVersionAppsBoundByPackages(
      spaceId.value,
      currentTemplateSpace.value,
      template_set_ids,
      params,
    );
    citedList.value = res.details;
    citeByAppIds.value = citedList.value.map((Item) => Item.app_id);
    tableLoading.value = false;
  };

  const goToConfigPageImport = (id: number) => {
    const { href } = router.resolve({
      name: 'service-config',
      params: { appId: id },
      query: { pkg_id: currentTemplateSpace.value },
    });
    window.open(href, '_blank');
  };

  const handleConfirm = () => {
    const defaultVal = {
      privilege: '',
      user: '',
      user_group: '',
      uid: undefined,
      gid: undefined,
    };
    if (isEqual(localVal.value, defaultVal)) {
      emits('update:show', false);
    } else {
      emits('confirm', { permission: localVal.value, appIds: citeByAppIds.value });
    }
  };
</script>

<style scoped lang="scss">
  .perm-input {
    display: flex;
    align-items: center;
    width: 172px;
    :deep(.bk-input) {
      width: 140px;
      border-right: none;
      border-top-right-radius: 0;
      border-bottom-right-radius: 0;
      .bk-input--number-control {
        display: none;
      }
    }
    .perm-panel-trigger {
      width: 32px;
      height: 32px;
      text-align: center;
      background: #fafcfe;
      color: #3a84ff;
      border: 1px solid #3a84ff;
      cursor: pointer;
      &.disabled {
        color: #dcdee5;
        border-color: #dcdee5;
        cursor: not-allowed;
      }
    }
  }
  .privilege-select-panel {
    display: flex;
    align-items: top;
    border: 1px solid #dcdee5;
    .group-item {
      .header {
        padding: 0 16px;
        height: 42px;
        line-height: 42px;
        color: #313238;
        font-size: 12px;
        background: #fafbfd;
        border-bottom: 1px solid #dcdee5;
      }
      &:not(:last-of-type) {
        .header,
        .checkbox-area {
          border-right: 1px solid #dcdee5;
        }
      }
    }
    .checkbox-area {
      padding: 10px 16px 12px;
      background: #ffffff;
      &:not(:last-child) {
        border-right: 1px solid #dcdee5;
      }
    }
    .group-checkboxs {
      font-size: 12px;
      .bk-checkbox ~ .bk-checkbox {
        margin-left: 16px;
      }
      :deep(.bk-checkbox-label) {
        font-size: 12px;
      }
    }
  }
  .selected-tag {
    display: inline-block;
    height: 32px;
    background: #f0f1f5;
    line-height: 32px;
    border-radius: 16px;
    padding: 0 12px;
    margin: 8px 0px 16px;
    .count {
      color: #3a84ff;
    }
  }
</style>

<style lang="scss">
  .batch-operation-button-popover.bk-popover.bk-pop2-content {
    padding: 4px 0;
    border: 1px solid #dcdee5;
    box-shadow: 0 2px 6px 0 #0000001a;
    .operation-item {
      padding: 0 12px;
      min-width: 58px;
      height: 32px;
      line-height: 32px;
      color: #63656e;
      font-size: 12px;
      cursor: pointer;
      &:hover {
        background: #f5f7fa;
      }
    }
  }
  .app-info {
    display: flex;
    align-items: center;
    overflow: hidden;
    cursor: pointer;
    .name-text {
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }
    .link-icon {
      flex-shrink: 0;
      margin-left: 10px;
    }
  }
</style>
