<template>
  <bk-dialog
    :title="`${t('上线版本')}-${props.version ? props.version : versionData.spec.name}`"
    ext-cls="release-version-dialog"
    :is-show="props.show"
    :esc-close="false"
    :quick-close="false"
    :is-loading="pending"
    @closed="handleClose"
    @confirm="handleConfirm">
    <bk-form class="form-wrapper" form-type="vertical" ref="formRef" :rules="rules" :model="localVal">
      <template v-if="props.releaseType === 'all'">
        <div v-if="excludeGroups.length > 0" class="exclude-groups">
          <p class="tips">
            {{ t('确认上线后，以下分组') }}
            <span class="em">{{ t('以外') }}</span>
            {{ t('的客户端实例将上线当前版本配置') }}
          </p>
          <div class="group-list-wrapper">
            <div v-for="group in excludeGroups" class="group-item" :key="group.id">
              <div class="name">{{ group.name }}</div>
              <div v-if="group.rules.length > 0" class="rules">
                <bk-overflow-title type="tips">
                  <span v-for="(rule, index) in group.rules" :key="index" class="rule">
                    <span v-if="index > 0"> & </span>
                    <rule-tag class="tag-item" :rule="rule" />
                  </span>
                </bk-overflow-title>
              </div>
            </div>
          </div>
        </div>
        <p v-else class="tips">
          {{ t('确认上线后，') }}
          <span class="em">{{ t('全部') }}</span>
          {{ t('客户端实例将上线当前版本配置') }}
        </p>
      </template>
      <bk-form-item
        v-if="previewData.length > 0"
        :label="
          props.releaseType === 'select'
            ? t('确认上线后，以下分组的客户端实例将上线当前版本配置')
            : t('以下分组将变更版本')
        ">
        <div class="group-list-wrapper">
          <div v-for="previewGroup in previewData" class="release-section" :key="previewGroup.id">
            <div class="section-header" @click="previewGroup.fold = !previewGroup.fold">
              <span class="angle-icon">
                <AngleRight v-if="previewGroup.fold" />
                <AngleDown v-else />
              </span>
              <div :class="['version-type-marking', previewGroup.type]">
                【{{ TYPE_MAP[previewGroup.type as keyof typeof TYPE_MAP] }}】
              </div>
              <span v-if="previewGroup.type === 'modify'" class="release-name">
                {{ previewGroup.name }} <ArrowsRight class="arrow-icon" />
                {{ props.version ? props.version : versionData.spec.name }}
              </span>
            </div>
            <div v-show="!previewGroup.fold" class="group-list">
              <div v-for="group in previewGroup.children" class="group-item" :key="group.id">
                <div class="name">{{ group.name }}</div>
                <div v-if="group.desc" class="desc">{{ group.desc }}</div>
                <div v-if="group.rules.length > 0" class="rules">
                  <bk-overflow-title type="tips">
                    <span v-for="(rule, index) in group.rules" :key="index" class="rule">
                      <span v-if="index > 0"> & </span>
                      <rule-tag class="tag-item" :rule="rule" />
                    </span>
                  </bk-overflow-title>
                </div>
              </div>
            </div>
          </div>
        </div>
      </bk-form-item>
      <bk-form-item :label="t('上线说明')" property="memo">
        <bk-input
          v-model="localVal.memo"
          type="textarea"
          :disabled="props.secondConfirm"
          :placeholder="props.secondConfirm ? ' ' : t('请输入')"
          :maxlength="200"
          :resize="true" />
      </bk-form-item>
      <bk-form-item v-if="!props.secondConfirm" property="publish_time">
        <template #label>
          <span>{{ t('上线方式') }}</span>
          <help-fill
            v-bk-tooltips="{
              content: publishTip,
              placement: 'top-start',
              theme: 'dark',
            }"
            class="mode-tip" />
        </template>
        <bk-loading :loading="pending">
          <bk-radio-group v-model="localVal.publish_type" :class="{ 'publish-type-wrap': locale !== 'zh-cn' }">
            <!-- 未开启审批 -->
            <template v-if="!isApprove">
              <bk-radio label="immediately">{{ t('立即上线') }}</bk-radio>
              <bk-radio label="scheduled">{{ t('定时上线') }}</bk-radio>
            </template>
            <!-- 开启审批 -->
            <template v-else>
              <bk-radio label="manually">{{ t('手动上线') }}</bk-radio>
              <bk-radio label="automatically">{{ t('审批通过后立即上线') }}</bk-radio>
              <bk-radio label="scheduled">{{ t('定时上线') }}</bk-radio>
            </template>
          </bk-radio-group>
          <bk-date-picker
            ref="datePickerRef"
            v-show="localVal.publish_type === 'scheduled'"
            v-model="localVal.publish_time"
            append-to-body
            type="datetime"
            ext-popover-cls="date-picker-popover"
            placement="top-start"
            :editable="false"
            :clearable="false"
            :disabled-date="disabledDate"
            :open="datePickerShow"
            @open-change="datePickerShow = true"
            @pick-success="datePickerShow = false">
            <template #header>
              <div v-if="isTimeMode" @click="getCurrentTime" data-no-close="true">此刻</div>
            </template>
          </bk-date-picker>
        </bk-loading>
      </bk-form-item>
    </bk-form>
    <template #footer>
      <div class="dialog-footer">
        <bk-button v-if="props.secondConfirm" theme="primary" @click="handleSecondConfirm">{{ t('上线') }}</bk-button>
        <bk-button v-else theme="primary" :loading="pending" @click="handleConfirm">
          {{ isApprove ? t('提交上线审批') : t('确认上线') }}
        </bk-button>
        <bk-button :disabled="pending" @click="handleClose">{{ t('取消') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>
<script setup lang="ts">
  import { computed, ref, watch } from 'vue';
  import { storeToRefs } from 'pinia';
  import { useI18n } from 'vue-i18n';
  import { AngleDown, AngleRight, ArrowsRight, HelpFill } from 'bkui-vue/lib/icon';
  import { publishVerSubmit, publishType } from '../../../../../../api/config';
  import { IGroupToPublish, IGroupPreviewItem } from '../../../../../../../types/group';
  import useConfigStore from '../../../../../../store/config';
  import { aggregatePreviewData, aggregateExcludedData } from '../hooks/aggregate-groups';
  import RuleTag from '../../../../groups/components/rule-tag.vue';
  import dayjs from 'dayjs';
  import { convertTime } from '../../../../../../utils';

  const versionStore = useConfigStore();
  const { versionData } = storeToRefs(versionStore);

  const { t, locale } = useI18n();

  interface IFormData {
    groups: number[];
    all: boolean;
    memo: string;
    publish_type: 'manually' | 'automatically' | 'scheduled' | 'immediately' | '';
    publish_time: Date | string;
  }

  interface IModifyReleasePreviewItem extends IGroupPreviewItem {
    fold: boolean;
  }

  const TYPE_MAP = {
    plain: t('首次上线'),
    modify: t('变更版本'),
    retain: t('保留版本'),
  };

  const props = withDefaults(
    defineProps<{
      show: boolean;
      bkBizId: string;
      appId: number;
      groupList: IGroupToPublish[];
      releaseType: string;
      releasedGroups?: number[];
      groups: IGroupToPublish[];
      secondConfirm?: boolean;
      memo?: string;
      version?: string;
    }>(),
    {
      releasedGroups: () => [],
      secondConfirm: false,
      memo: '',
    },
  );

  const emits = defineEmits(['confirm', 'update:show', 'secondConfirm']);

  const localVal = ref<IFormData>({
    groups: [],
    all: false,
    memo: '',
    publish_type: '',
    // publish_time: dayjs().add(2, 'hour').format('YYYY-MM-DD HH:mm:ss'), // 默认当前时间的后两小时
    publish_time: '',
  });
  const previewData = ref<IModifyReleasePreviewItem[]>([]);
  const excludeGroups = ref<IGroupToPublish[]>([]);
  const pending = ref(false);
  const formRef = ref();
  const isApprove = ref(false); // 服务的审批状态
  const datePickerRef = ref();
  const datePickerShow = ref(false);

  const rules = {
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
    publish_time: [
      {
        validator: (value: Date) => {
          if (localVal.value.publish_type === 'scheduled' && !value) {
            return false;
          }
          return true;
        },
        message: t('请选择'),
      },
      {
        validator: (value: Date) => {
          if (localVal.value.publish_type === 'scheduled' && dayjs().isAfter(dayjs(value))) {
            return false;
          }
          return true;
        },
        message: t('不能选择过去的时间'),
      },
    ],
  };

  const publishTip = computed(() => {
    return isApprove.value ? t('审批开启的文案') : t('审批关闭的文案');
  });

  const isCompare = computed(() => previewData.value.some((item) => item.type !== 'plain'));

  const isTimeMode = computed(() => {
    return datePickerRef.value && datePickerRef.value.selectionMode === 'time';
  });

  watch(
    () => props.show,
    (val) => {
      if (val) {
        const previewList = aggregatePreviewData(
          props.groups,
          props.groupList,
          props.releasedGroups,
          props.releaseType,
          versionData.value.id,
        );
        previewData.value = previewList.map((item) => ({ ...item, fold: false }));
        const excludeList = aggregateExcludedData(
          props.groups,
          props.groupList,
          props.releaseType,
          versionData.value.id,
        );
        const list: IGroupToPublish[] = [];
        excludeList.forEach((item) => {
          list.push(...item.children);
        });
        excludeGroups.value = list;
        loadPublishType();
        if (props.secondConfirm) {
          localVal.value.memo = props.memo;
        }
      }
    },
  );

  watch(
    () => props.groups,
    () => {
      localVal.value.groups = props.groups.map((item) => item.id);
    },
    { immediate: true },
  );

  const disabledDate = (date: any) => {
    return date && dayjs(date).isBefore(dayjs().subtract(1, 'day'));
  };

  const handleClose = () => {
    emits('update:show', false);
    localVal.value = {
      groups: [],
      all: false,
      memo: '',
      publish_type: '',
      publish_time: '',
    };
    datePickerShow.value = false;
  };

  const handleConfirm = async () => {
    try {
      pending.value = true;
      await formRef.value.validate();
      const params = { ...localVal.value, is_compare: isCompare.value };
      // 全部实例上线，只需要将all置为true
      if (props.releaseType === 'all') {
        if (excludeGroups.value.length > 0) {
          params.all = false;
        } else {
          params.all = true;
          params.groups = [];
        }
      }
      // 非定时上线，publishTime清空
      params.publish_time =
        localVal.value.publish_type === 'scheduled' ? convertTime(params.publish_time as string, 'utc') : '';
      const resp = await publishVerSubmit(props.bkBizId, props.appId, versionData.value.id, params);
      handleClose();
      // 目前组件库dialog关闭自带250ms的延迟，所以这里延时300ms
      setTimeout(() => {
        emits(
          'confirm',
          resp.data.have_pull as boolean,
          isApprove.value,
          params.publish_type,
          convertTime(params.publish_time as string, 'local'),
        );
      }, 300);
    } catch (e) {
      console.error(e);
      // InfoBox({
      // // @ts-ignore
      //   infoType: "danger",
      //   title: '版本上线失败',
      //   subTitle: e.response.data.error.message,
      //   confirmText: '重试',
      //   onConfirm () {
      //     handleConfirm()
      //   }
      // })
    } finally {
      pending.value = false;
    }
  };

  const handleSecondConfirm = () => {
    handleClose();
    setTimeout(() => {
      emits('secondConfirm');
    }, 300);
  };

  const getCurrentTime = () => {
    const hour = dayjs().hour();
    const minute = dayjs().minute();
    const second = dayjs().second();
    localVal.value.publish_time = dayjs(localVal.value.publish_time)
      .set('hour', hour)
      .set('minute', minute)
      .set('second', second)
      .toDate();
  };

  const loadPublishType = async () => {
    try {
      pending.value = true;
      const resp = await publishType(props.bkBizId, props.appId);
      const { is_approve, publish_type } = resp.data;
      isApprove.value = is_approve;
      // 需要审批
      if (is_approve) {
        localVal.value.publish_type = ['manually', 'automatically', 'scheduled'].includes(publish_type)
          ? publish_type
          : 'manually';
      } else {
        // 不需要审批，默认选择选项的第一个
        localVal.value.publish_type = ['immediately', 'scheduled'].includes(publish_type)
          ? publish_type
          : 'immediately';
      }
    } catch (error) {
      console.log(error);
      // 产品要求数据不对时默认选中立即上线
      localVal.value.publish_type = 'immediately';
    } finally {
      pending.value = false;
    }
  };
</script>
<style lang="scss" scoped>
  .form-wrapper {
    padding-bottom: 24px;
    :deep(.bk-form-label) {
      font-size: 12px;
    }
  }
  .exclude-groups {
    margin-bottom: 16px;
    .tips {
      display: flex;
      align-items: center;
      margin: 0 0 8px;
      font-size: 12px;
      line-height: 20px;
      .em {
        font-weight: 700;
        color: #ff9c01;
      }
    }
  }
  .group-list-wrapper {
    padding: 8px;
    max-height: 320px;
    border: 1px solid #dcdee5;
    border-radius: 2px;
    overflow: auto;
    .release-section {
      margin-bottom: 8px;
    }
    .section-header {
      display: flex;
      align-items: center;
      font-size: 12px;
      color: #63656e;
      cursor: pointer;
      &:hover {
        .angle-icon {
          color: #3a84ff;
        }
      }
      .angle-icon {
        font-size: 18px;
        line-height: 1;
      }
      .version-type-marking {
        &.modify {
          color: #ff9c01;
        }
      }
      .release-name {
        display: inline-flex;
        align-items: center;
        .arrow-icon {
          font-size: 20px;
          color: #ff9c01;
        }
      }
    }
  }
  .group-item {
    display: flex;
    align-items: center;
    white-space: nowrap;
    overflow: hidden;
    &:not(:last-child) {
      margin-bottom: 8px;
    }
    .name {
      padding: 0 10px;
      height: 22px;
      line-height: 22px;
      font-size: 12px;
      color: #63656e;
      background: #f0f1f5;
      border-radius: 2px;
    }
    .desc {
      font-size: 12px;
      color: #979ba5;
    }
    .rules {
      margin-left: 8px;
      font-size: 12px;
      line-height: 22px;
      color: #c4c6cc;
      overflow: hidden;
    }
  }
  .dialog-footer {
    .bk-button {
      margin-left: 8px;
    }
  }
  .mode-tip {
    margin-left: 9px;
    vertical-align: middle;
    font-size: 14px;
    color: #979ba5;
    cursor: pointer;
  }
  .publish-type-wrap {
    flex-wrap: wrap;
    .bk-radio:nth-child(3) {
      margin-left: 0;
    }
  }
</style>
<style lang="scss">
  .release-version-dialog.bk-modal-wrapper .bk-dialog-header {
    padding-bottom: 20px;
  }
  .date-picker-popover {
    .bk-date-picker-top-wrapper {
      position: absolute;
      right: 54px;
      top: 22px;
      color: #3a84ff;
      cursor: pointer;
    }
  }
</style>
