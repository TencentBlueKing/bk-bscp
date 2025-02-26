<template>
  <section>
    <div class="record-table-wrapper">
      <bk-loading style="min-height: 300px" :loading="loading">
        <bk-table
          class="record-table"
          show-overflow-tooltip
          :row-height="0"
          :border="['outer']"
          :data="tableData"
          @column-sort="handleSort"
          @column-filter="handleFilter">
          <bk-table-column :label="t('操作时间')" width="155" :sort="true">
            <template #default="{ row }">
              {{ convertTime(row.audit?.revision.created_at, 'local') }}
            </template>
          </bk-table-column>
          <bk-table-column :label="t('所属服务')" width="180">
            <template #default="{ row }"> {{ row.app?.name || '--' }} </template>
          </bk-table-column>
          <bk-table-column
            :label="t('资源类型')"
            :width="locale === 'zh-cn' ? '96' : '160'"
            :filter="{
              filterFn: () => true,
              list: resTypeFilterList,
              checked: resTypeFilterChecked,
            }">
            <template #default="{ row }">
              {{ RECORD_RES_TYPE[row.audit?.spec.res_type as keyof typeof RECORD_RES_TYPE] || '--' }}
            </template>
          </bk-table-column>
          <bk-table-column
            :label="t('操作行为')"
            :width="locale === 'zh-cn' ? '114' : '240'"
            :filter="{
              filterFn: () => true,
              list: actionFilterList,
              checked: actionFilterChecked,
            }">
            <template #default="{ row }">
              {{ ACTION[row.audit?.spec.action as keyof typeof ACTION] || '--' }}
            </template>
          </bk-table-column>
          <bk-table-column :label="t('资源实例')" min-width="363">
            <template #default="{ row }">
              <div
                v-if="row.audit && row.audit.spec.res_instance"
                v-html="convertInstance(row.audit.spec.res_instance)"
                class="multi-line-styles" />
              <div v-else class="multi-line-styles">--</div>
              <!-- <div>{{ row.audit?.spec.res_instance || '--' }}</div> -->
            </template>
          </bk-table-column>
          <bk-table-column :label="t('操作人')" width="140">
            <template #default="{ row }">
              {{ row.audit?.spec.operator || '--' }}
            </template>
          </bk-table-column>
          <bk-table-column :label="t('操作途径')" :width="locale === 'zh-cn' ? '90' : '150'">
            <template #default="{ row }"> {{ row.audit?.spec.operate_way || '--' }} </template>
          </bk-table-column>
          <bk-table-column
            :label="t('状态')"
            :show-overflow-tooltip="false"
            :width="locale === 'zh-cn' ? '130' : '190'"
            :filter="{
              filterFn: () => true,
              list: approveStatusFilterList,
              checked: approveStatusFilterChecked,
            }">
            <template #default="{ row }">
              <template v-if="row.audit?.spec.status">
                <div :class="['dot', ...setApprovalClass(row.audit.spec.status)]"></div>
                {{ STATUS[row.audit.spec.status as keyof typeof STATUS] || '--' }}
                <!-- 上线时间icon -->
                <div
                  v-if="row.strategy?.publish_time && row.audit.spec.status === APPROVE_STATUS.pending_publish"
                  v-bk-tooltips="{
                    content: `${t('定时上线')}: ${convertTime(row.strategy.publish_time, 'local')}${
                      isTimeout(row.strategy.publish_time) ? `(${t('已过时')})` : ''
                    }`,
                    placement: 'top',
                  }"
                  class="time-icon"></div>
                <!-- 信息提示icon：待审批/审批驳回样式 -->
                <bk-popover :popover-delay="[0, 300]" placement="bottom-end" theme="light">
                  <text-file
                    v-if="
                      [APPROVE_STATUS.pending_approval, APPROVE_STATUS.rejected_approval].includes(
                        row.audit.spec.status,
                      )
                    "
                    class="info-line is-text" />
                  <template #content>
                    <div class="popover-content">
                      <template v-if="row.strategy.itsm_ticket_sn">
                        <div>{{ $t('审批单') }}：</div>
                        <div class="itsm-content em">
                          <div class="itsm-sn" @click="handleLinkTo(row.strategy.itsm_ticket_url)">
                            {{ row.strategy.itsm_ticket_sn }}
                          </div>
                          <div class="itsm-action" @click="handleCopy(row.strategy.itsm_ticket_url)"><Copy /></div>
                        </div>
                      </template>
                      <div class="itsm-title">
                        {{ $t('审批人') }}
                        ({{ row.app.approve_type === 'or_sign' ? $t('或签') : $t('会签') }})：
                      </div>
                      <div class="itsm-content">
                        {{ row.strategy.approver_progress }}
                      </div>
                      <template v-if="row.audit.spec.status === APPROVE_STATUS.rejected_approval">
                        <div class="itsm-title">{{ $t('审批时间') }}：</div>
                        <div class="itsm-content">
                          {{ convertTime(row.strategy.final_approval_time, 'local') || '--' }}
                        </div>
                      </template>
                      <template
                        v-if="row.audit.spec.status === APPROVE_STATUS.pending_approval && row.strategy.publish_time">
                        <div class="itsm-title">{{ $t('定时上线') }}：</div>
                        <div class="itsm-content">
                          {{ convertTime(row.strategy.publish_time, 'local') || '--' }}
                        </div>
                      </template>
                      <template v-if="row.audit.spec.status === APPROVE_STATUS.rejected_approval">
                        <div class="itsm-title">{{ $t('驳回原因') }}：</div>
                        <div class="itsm-content">
                          {{ row.strategy.reject_reason }}
                        </div>
                      </template>
                    </div>
                  </template>
                </bk-popover>
                <!-- 信息提示icon：已上线/已撤销/失败样式 -->
                <info-line
                  v-if="
                    [APPROVE_STATUS.already_publish, APPROVE_STATUS.revoked_publish, APPROVE_STATUS.failure].includes(
                      row.audit.spec.status,
                    )
                  "
                  v-bk-tooltips="{
                    content: statusTip(row),
                    placement: 'top',
                  }"
                  class="info-line" />
              </template>
              <template v-else>--</template>
            </template>
          </bk-table-column>
          <bk-table-column
            fixed="right"
            :show-overflow-tooltip="false"
            :label="t('操作')"
            :width="locale === 'zh-cn' ? '160' : '260'">
            <template #default="{ row }">
              <!-- 仅上线配置版本存在待审批或待上线等状态和相关操作 -->
              <div v-if="row.audit && row.audit.spec.action === 'publish'" class="action-btns">
                <!-- 创建者且版本待上线 才展示上线按钮;审批通过的时间在定时上线的时间以前，上线按钮置灰 -->
                <bk-button
                  v-if="
                    row.audit.spec.status === APPROVE_STATUS.pending_publish &&
                    (!row.strategy.publish_time || isTimeout(row.strategy.publish_time))
                  "
                  v-bk-tooltips="{
                    content: $t('无确认上线权限文案', { creator: row.strategy.creator }),
                    placement: 'top',
                    disabled: row.strategy.creator === userInfo.username,
                  }"
                  class="action-btn"
                  text
                  theme="primary"
                  :disabled="row.strategy.creator !== userInfo.username"
                  @click="handlePublishClick(row)">
                  {{ t('确认上线') }}
                </bk-button>
                <!-- 1.待审批状态 且 对应审批人才可显示 -->
                <!-- 2.版本首次在分组上线的情况，显示审批，点击审批直接通过 -->

                <template
                  v-else-if="
                    row.audit.spec.status === APPROVE_STATUS.pending_approval &&
                    row.strategy.approver_progress.includes(userInfo.username)
                  ">
                  <!--
                    row.audit.spec.is_compare：true需要对比(非首次上线)
                    目标分组非首次上线，打开对比抽屉
                    目标分组首次上线，打开版本详情抽屉
                   -->
                  <bk-button
                    class="action-btn"
                    text
                    theme="primary"
                    @click="handleApproval(row, !row.audit.spec.is_compare)">
                    {{ t('去审批') }}
                  </bk-button>
                  <!-- 目标分组首次上线，打开版本详情抽屉 -->
                </template>
                <!-- 审批驳回/已撤销才可显示 -->
                <bk-button
                  v-else-if="
                    [APPROVE_STATUS.rejected_approval, APPROVE_STATUS.revoked_publish].includes(row.audit.spec.status)
                  "
                  class="action-btn"
                  text
                  theme="primary"
                  @click="retrySubmission(row)">
                  {{ t('再次提交') }}
                </bk-button>
                <span
                  v-else-if="
                    ![APPROVE_STATUS.pending_approval, APPROVE_STATUS.pending_publish].includes(row.audit.spec.status)
                  "
                  class="empty-action">
                  --
                </span>
                <!-- 待上线/去审批状态且版本创建者才显示撤销 -->
                <bk-button
                  v-if="
                    [APPROVE_STATUS.pending_approval, APPROVE_STATUS.pending_publish].includes(row.audit.spec.status)
                  "
                  text
                  class="action-btn"
                  theme="primary"
                  @click="handleConfirm(row)">
                  {{ $t('撤销上线') }}
                </bk-button>
                <!-- <MoreActions
                  v-if="
                    [APPROVE_STATUS.pending_approval, APPROVE_STATUS.pending_publish].includes(row.audit.spec.status) &&
                    row.strategy.creator === userInfo.username
                  "
                  @handle-undo="handleConfirm(row, $event)" /> -->
              </div>
              <template v-else>--</template>
            </template>
          </bk-table-column>
          <template #empty>
            <TableEmpty :is-search-empty="isSearchEmpty" />
          </template>
        </bk-table>
        <bk-pagination
          v-model="pagination.current"
          class="table-list-pagination"
          location="left"
          :limit="pagination.limit"
          :layout="['total', 'limit', 'list']"
          :count="pagination.count"
          @change="handlePageChange"
          @limit-change="handlePageLimitChange" />
      </bk-loading>
    </div>
    <!-- 撤销弹窗 -->
    <RepealDialog
      v-model:show="repealDialogShow"
      :space-id="spaceId"
      :app-id="rowAppId"
      :release-id="rowReleaseId"
      :data="confirmData"
      @refresh-list="loadRecordList" />
    <PublishDialog
      v-model:show="publishDialogShow"
      :bk-biz-id="spaceId"
      :app-id="confirmData.serviceId"
      :group-list="groupList"
      :groups="groups"
      :release-type="releaseType"
      :second-confirm="true"
      :memo="confirmData.memo"
      :version="confirmData.version"
      @second-confirm="handleConfirmPublish" />
    <!-- 审批对比抽屉 -->
    <VersionDiff
      :show="approvalShow"
      :space-id="spaceId"
      :app-id="rowAppId"
      :release-id="rowReleaseId"
      :released-groups="rowReleaseGroups"
      @close="closeApprovalDialog" />
    <!-- 首次目标分组上线审批信息展示抽屉 -->
    <VersionInfo
      :show="firstApprovalShow"
      :space-id="spaceId"
      :app-id="rowAppId"
      :release-id="rowReleaseId"
      :released-groups="rowReleaseGroups"
      @close="closeApprovalDialog" />
  </section>
</template>

<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { useRouter, useRoute } from 'vue-router';
  import { debounce } from 'lodash';
  import { useI18n } from 'vue-i18n';
  import { IRecordQuery, IDialogData, IRowData } from '../../../../../types/record';
  import { RECORD_RES_TYPE, ACTION, STATUS, INSTANCE, APPROVE_STATUS, ONLINE_TYPE } from '../../../../constants/record';
  import { storeToRefs } from 'pinia';
  import useUserStore from '../../../../store/user';
  import { getRecordList, approve } from '../../../../api/record';
  import useTablePagination from '../../../../utils/hooks/use-table-pagination';
  import TableEmpty from '../../../../components/table/table-empty.vue';
  import RepealDialog from './dialog-confirm.vue';
  import { InfoLine, Copy, TextFile } from 'bkui-vue/lib/icon';
  import VersionDiff from './version-diff.vue';
  import VersionInfo from './version-info.vue';
  import BkMessage from 'bkui-vue/lib/message';
  import { convertTime, copyToClipBoard } from '../../../../utils';
  import { getServiceGroupList } from '../../../../api/group';
  import { IGroupItemInService, IGroupToPublish } from '../../../../../types/group';
  import dayjs from 'dayjs';
  import PublishDialog from '../../service/detail/components/publish-version/confirm-dialog.vue';
  import { InfoBox } from 'bkui-vue';

  const props = withDefaults(
    defineProps<{
      spaceId: string;
      searchParams: IRecordQuery;
    }>(),
    {
      spaceId: '',
    },
  );

  const emits = defineEmits(['handle-table-filter']);

  const router = useRouter();
  const route = useRoute();
  const { t, locale } = useI18n();
  const { userInfo } = storeToRefs(useUserStore());
  const { pagination, updatePagination } = useTablePagination('recordList');

  const loading = ref(true);
  const isSearchEmpty = ref(false);
  const searchParams = ref<IRecordQuery>({});
  const actionTimeSrotMode = ref('');
  const tableData = ref<IRowData[]>([]);
  const approvalShow = ref(false);
  const firstApprovalShow = ref(false);
  const rowAppId = ref(-1);
  const rowReleaseId = ref(-1);
  const rowReleaseGroups = ref<number[]>([]);
  const repealDialogShow = ref(false);
  const publishDialogShow = ref(false);
  const confirmData = ref<IDialogData>({
    service: '',
    version: '',
    group: '',
    serviceId: 0,
    releaseId: 0,
    memo: '',
  });
  const groupList = ref<IGroupToPublish[]>([]);
  const groups = ref<IGroupToPublish[]>([]);
  const releaseType = ref('select');

  // 数据过滤 S
  // 1. 资源类型
  const resTypeFilterChecked = ref<string[]>([]);
  const resTypeFilterList = Object.entries(RECORD_RES_TYPE).map(([key, value]) => ({
    text: value,
    value: key,
  }));
  // 2. 操作行为
  const actionFilterChecked = ref<string[]>([]);
  const actionFilterList = Object.entries(ACTION).map(([key, value]) => ({
    text: value,
    value: key,
  }));
  // 3. 状态
  const approveStatusFilterChecked = ref<string[]>([]);
  const approveStatusFilterList = Object.entries(STATUS).map(([key, value]) => ({
    text: value,
    value: key,
  }));
  // 数据过滤 E

  watch(
    () => props.searchParams,
    (newV) => {
      searchParams.value = {
        ...newV,
      };
      searchParams.value.all = !(route.params.appId && Number(route.params.appId) > -1);
      if (searchParams.value.all) {
        delete searchParams.value.app_id;
      } else {
        searchParams.value.app_id = Number(route.params.appId);
      }
      loadRecordList();
    },
    { deep: true },
  );

  watch(
    () => route.params.appId,
    (newV) => {
      searchParams.value.all = !(newV && Number(newV) > -1);
      if (searchParams.value.all) {
        delete searchParams.value.app_id;
      } else {
        searchParams.value.app_id = Number(route.params.appId);
      }
      delete searchParams.value.id;
      loadRecordList();
    },
  );

  // 加载操作记录列表数据
  const loadRecordList = async () => {
    try {
      loading.value = true;
      const { start_time, end_time } = searchParams.value;
      const params: IRecordQuery = {
        start: pagination.value.limit * (pagination.value.current - 1),
        limit: Number(route.query.limit) || pagination.value.limit,
        ...searchParams.value,
        start_time: start_time ? convertTime(start_time!, 'utc') : '',
        end_time: end_time ? convertTime(end_time!, 'utc') : '',
      };
      const res = await getRecordList(props.spaceId, params);
      tableDataSort(res.details);
      pagination.value.count = res.count;
      // 是否打开审批抽屉
      if (route.query.id) {
        openApprovalSideBar();
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading.value = false;
    }
  };

  // 关闭审批对比弹窗
  const closeApprovalDialog = (refresh: string) => {
    approvalShow.value = false;
    firstApprovalShow.value = false;
    // 去除url操作记录id
    if (route.query.id) {
      const newQuery = { ...route.query };
      delete newQuery.id;
      router.replace({
        query: {
          ...newQuery,
        },
      });
    }
    // 审批通过/驳回：刷新
    if (refresh) {
      loadRecordList();
    }
  };

  // 资源示例映射
  const convertInstance = (data: string) => {
    if (!data.length) return '';

    let resultList = data.split('\n');
    let operateCount: string | null = null;
    let operateIndex: number | null = null;

    // 提取操作对象的个数及其索引
    resultList.forEach((result, index) => {
      const match = result.match(/operate_objects: (\d+)/);
      if (match) {
        operateCount = match[1];
        operateIndex = index;
      }
    });

    if (operateIndex !== null && operateCount) {
      resultList.splice(operateIndex, 1);
    }

    resultList = resultList.map((result) => {
      const [key, ...rest] = result.split(':');
      const mappedKey = INSTANCE[key as keyof typeof INSTANCE] || key;
      return [mappedKey, ...rest].join(':');
    });

    if (operateCount && operateIndex !== null) {
      const operationDescription = t('对{n}等 {m} 个对象进行操作', {
        n: resultList[operateIndex],
        m: operateCount,
      });
      resultList.splice(operateIndex, 1, operationDescription);
    }

    return resultList.join('<br />');
  };

  // 状态提示信息
  const statusTip = (row: IRowData) => {
    if (!row) {
      return '--';
    }
    const { status, detail } = row.audit.spec;
    // const approveType = row.app.approve_type === 'or_sign' ? t('或签') : t('会签');
    const {
      final_approval_time: time,
      reviser,
      reject_reason: reason,
      memo,
      final_approval_time: publish_time,
    } = row.strategy;
    switch (status) {
      // case APPROVE_STATUS.pending_approval:
      //   return t('提示-待审批', { approver_progress, approveType });
      case APPROVE_STATUS.already_publish:
        return t('提示-已上线文案', { time: convertTime(publish_time, 'local'), reviser, memo: memo || '--' });
      // case APPROVE_STATUS.rejected_approval:
      //   return t('提示-审批驳回', {
      //     reviser,
      //     time: convertTime(time, 'local'),
      //     reason,
      //   });
      case APPROVE_STATUS.revoked_publish:
        return t('提示-已撤销', { reviser, time: convertTime(time, 'local'), reason: reason || '--' });
      case APPROVE_STATUS.failure:
        return detail;
      default:
        return '--';
    }
  };

  // 复制审批链接
  const handleCopy = (str?: string) => {
    if (!str) return;
    copyToClipBoard(str);
    BkMessage({
      theme: 'success',
      message: t('ITSM 审批链接已复制！'),
    });
  };

  // 上线时间是否超时
  const isTimeout = (time: string) => {
    const currentTime = dayjs();
    const publishTime = dayjs(convertTime(time, 'local'));
    // 定时的上线时间是否在当前时间之前
    return publishTime.isBefore(currentTime);
  };

  // 撤回提示框
  const handleConfirm = (row: IRowData) => {
    repealDialogShow.value = true;
    const matchVersion = row.audit.spec.res_instance.match(/releases_name:([^\n]*)/);
    const matchGroup = row.audit.spec.res_instance.match(/group:([^\n]*)/);
    rowAppId.value = row.audit.attachment.app_id;
    rowReleaseId.value = row.strategy.release_id;
    confirmData.value = {
      service: row.app.name,
      version: matchVersion ? matchVersion[1] : '--',
      group: matchGroup ? matchGroup[1] : '--',
      memo: '',
      serviceId: row.audit.attachment.app_id,
      releaseId: row.strategy.release_id,
    };
  };

  // 确认上线
  const handlePublishClick = async (row: IRowData) => {
    await getAllGroupData(row.audit.attachment.app_id);
    const publishGroupIds = row.strategy.scope.groups.map((group) => group.id);
    const matchVersion = row.audit.spec.res_instance.match(/releases_name:([^\n]*)/);
    if (publishGroupIds.length === 0) {
      groups.value = groupList.value;
      releaseType.value = 'all';
    } else {
      groups.value = groupList.value.filter((item) => publishGroupIds.includes(item.id));
      if (publishGroupIds.includes(0)) {
        releaseType.value = 'all';
      } else {
        releaseType.value = 'select';
      }
    }
    publishDialogShow.value = true;
    confirmData.value = {
      service: row.app.name,
      version: matchVersion ? matchVersion[1] : '--',
      group: '',
      memo: row.strategy.memo,
      serviceId: row.audit.attachment.app_id,
      releaseId: row.strategy.release_id,
    };
  };

  const handleConfirmPublish = async () => {
    const resp = await approve(props.spaceId, confirmData.value.serviceId, confirmData.value.releaseId, {
      publish_status: APPROVE_STATUS.already_publish,
    });
    loadRecordList();
    // 这里有两种情况且不会同时出现：
    // 1. itsm已经审批了，但我们产品页面还没有刷新
    // 2. itsm已经撤销了，但我们产品页面还没有刷新
    // 如果存在以上两种情况之一，提示使用message，否则继续后面流程
    const { message } = resp;
    if (message) {
      // 不再走上线流程
      BkMessage({
        theme: 'primary',
        message,
      });
    } else {
      // 继续上线流程
      handlePublishSuccess(resp.have_pull, false);
    }
  };

  // 版本上线成功/提交成功
  const handlePublishSuccess = (havePull: boolean, isApprove: boolean, publishType = '', publishTime = '') => {
    if (havePull || (!havePull && isApprove)) {
      InfoBox({
        infoType: 'success',
        'ext-cls': 'info-box-style',
        title: publishTitle(isApprove, publishType, publishTime),
        dialogType: 'confirm',
      });
    } else {
      InfoBox({
        infoType: 'success',
        title: publishTitle(isApprove, publishType, publishTime),
        'ext-cls': 'info-box-style',
        confirmText: t('配置客户端'),
        cancelText: t('稍后再说'),
        onConfirm: () => {
          const routeData = router.resolve({
            name: 'configuration-example',
            params: { spaceId: props.spaceId, appId: confirmData.value.serviceId },
          });
          window.open(routeData.href, '_blank');
        },
      });
    }
  };

  // 版本上线文案
  const publishTitle = (isApprove: boolean, type: string, time: string) => {
    switch (type) {
      case ONLINE_TYPE.manually:
        return t('手动上线文案');
      case ONLINE_TYPE.automatically:
        // return t('待审批通过后，调整分组将自动上线');
        return t('审批通过后上线文案');
      case ONLINE_TYPE.scheduled:
        return isApprove ? t('需审批-定时上线文案', { time }) : t('定时上线文案', { time });
      default:
        return t('版本已上线');
    }
  };

  // 获取所有上线服务内的分组列表，并组装tree组件节点需要的数据
  const getAllGroupData = async (appId: number) => {
    const res = await getServiceGroupList(props.spaceId, appId);
    groupList.value = res.details.map((group: IGroupItemInService) => {
      const { group_id, group_name, release_id, release_name } = group;
      const selector = group.new_selector;
      const rules = selector.labels_and || selector.labels_or || [];
      return { id: group_id, name: group_name, release_id, release_name, rules };
    });
  };

  // 再次提交
  const retrySubmission = (row: IRowData) => {
    const url = router.resolve({
      name: 'service-config',
      params: {
        appId: row.audit.attachment.app_id,
        versionId: row.strategy.release_id,
      },
    }).href;
    window.open(url, '_blank');
  };

  // 审批通过
  // const handleApproved = debounce(async (row: IRowData) => {
  //   try {
  //     const { biz_id, app_id } = row.audit.attachment;
  //     const { release_id } = row.strategy;
  //     const resp = await approve(String(biz_id), app_id, release_id, {
  //       publish_status: APPROVE_STATUS.pending_publish,
  //     });
  //     // 这里有两种情况且不会同时出现：
  //     // 1. itsm已经审批了，但我们产品页面还没有刷新
  //     // 2. itsm已经撤销了，但我们产品页面还没有刷新
  //     // 如果存在以上两种情况之一，提示使用message，否则message的值为空
  //     const { message } = resp;
  //     BkMessage({
  //       theme: message ? 'primary' : 'success',
  //       message: message ? t(message) : t('操作成功'),
  //     });
  //     loadRecordList();
  //   } catch (e) {
  //     console.log(e);
  //   }
  // }, 300);

  // 去审批
  const handleApproval = debounce(
    (row: IRowData, firstPublish = false) => {
      rowAppId.value = row.audit?.attachment.app_id;
      rowReleaseId.value = row.strategy?.release_id;
      // 当前row已上线版本的分组id,为空表示全部分组上线
      rowReleaseGroups.value = row.strategy.scope.groups.map((group) => group.id);
      // 目标分组是否首次上线
      if (firstPublish) {
        firstApprovalShow.value = true;
      } else {
        approvalShow.value = true;
      }
      router.replace({
        query: {
          ...route.query,
          id: row.audit.id,
        },
      });
    },
    300,
    { leading: true, trailing: false },
  );

  // 是否打开审批抽屉
  const openApprovalSideBar = () => {
    // 如果url的操作记录id为待审批状态，且为可对比状态并且当前登录用户有权限审批时，允许打开审批抽屉
    const isCompare = tableData.value[0]?.audit.spec.is_compare; // 是否可以对比版本不同
    const pendingApproval = tableData.value[0]?.strategy.publish_status === APPROVE_STATUS.pending_approval; // 是否待审批状态
    const isAuthorized = tableData.value[0]?.strategy.approver_progress.includes(userInfo.value.username); // 当前用户是否有权限审批
    if (pendingApproval && isAuthorized) {
      handleApproval(tableData.value[0], !isCompare);
    }
  };

  // 数据过滤
  const handleFilter = ({ checked, index }: any) => {
    // index: 2.资源类型 3.操作行为 7.状态
    switch (index) {
      case 2:
        searchParams.value.resource_type = checked.join(',');
        if (!checked.length) {
          delete searchParams.value.resource_type;
        }
        break;
      case 3:
        searchParams.value.action = checked.join(',');
        if (!checked.length) {
          delete searchParams.value.action;
        }
        break;
      case 7:
        searchParams.value.status = checked.join(',');
        if (!checked.length) {
          delete searchParams.value.status;
        }
        break;
      default:
        break;
    }
    emits('handle-table-filter', { ...searchParams.value }); // 使用表格过滤同步搜索框
    loadRecordList();
  };

  // 触发的排序模式
  const handleSort = ({ type }: any) => {
    actionTimeSrotMode.value = type === 'null' ? '' : type;
    tableDataSort(tableData.value);
  };

  // 列表排序
  const tableDataSort = (data: IRowData[]) => {
    if (actionTimeSrotMode.value === 'desc') {
      tableData.value = data.sort(
        (a, b) => dayjs(b.audit.revision.created_at).valueOf() - dayjs(a.audit.revision.created_at).valueOf(),
      );
    } else if (actionTimeSrotMode.value === 'asc') {
      tableData.value = data.sort(
        (a, b) => dayjs(a.audit.revision.created_at).valueOf() - dayjs(b.audit.revision.created_at).valueOf(),
      );
    } else {
      tableData.value = data;
    }
  };

  // 审批状态颜色
  const setApprovalClass = (status: APPROVE_STATUS) => {
    return [
      [APPROVE_STATUS.already_publish, APPROVE_STATUS.success].includes(status) ? 'green' : '',
      status === APPROVE_STATUS.pending_publish ? 'gray' : '',
      [APPROVE_STATUS.revoked_publish, APPROVE_STATUS.rejected_approval, APPROVE_STATUS.failure].includes(status)
        ? 'red'
        : '',
      status === APPROVE_STATUS.pending_approval ? 'orange' : '',
    ];
  };

  // 跳转审批页面
  const handleLinkTo = (url: string) => {
    if (url) {
      window.open(url, '_blank');
    }
  };

  //  翻页
  const handlePageChange = (val: number) => {
    pagination.value.current = val;
    loadRecordList();
  };

  const handlePageLimitChange = (val: number) => {
    updatePagination('limit', val);
    if (pagination.value.current === 1) {
      loadRecordList();
    }
  };
</script>

<style lang="scss" scoped>
  .record-table-wrapper {
    :deep(.bk-table-body) {
      max-height: calc(100vh - 280px);
      overflow: auto;
    }
    .dot {
      margin-right: 8px;
      display: inline-block;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      &.green {
        border: 1px solid #3fc06d;
        background-color: #e5f6ea;
      }
      &.gray {
        border: 1px solid #c4c6cc;
        background-color: #f0f1f5;
      }
      &.red {
        border: 1px solid #ea3636;
        background-color: #ffe6e6;
      }
      &.orange {
        border: 1px solid #ff9c01;
        background-color: #ffe8c3;
      }
    }
    // .status-text {
    //   display: inline-block;
    // }
    .time-icon {
      position: relative;
      margin-left: 8px;
      display: inline-block;
      width: 14px;
      height: 14px;
      vertical-align: bottom;
      border: 1px solid #3a84ff;
      border-radius: 50%;
      box-shadow: inset 0 0 0 0.1px #3a84ff;
      &::after {
        content: '';
        position: absolute;
        bottom: calc(50% - 1px);
        left: calc(50% - 1px);
        width: 35%;
        height: 35%;
        border-left: 1px solid #3a84ff;
        border-bottom: 1px solid #3a84ff;
      }
    }
    .info-line {
      margin-left: 8px;
      font-size: 15px;
      vertical-align: bottom;
      transform: scale(1.05);
      color: #979ba5;
      &.is-text {
        transform: scale(1);
      }
    }
  }
  .action-btns {
    position: relative;
    .action-btn + .action-btn {
      margin-left: 14px;
    }
  }
  .table-list-pagination {
    padding: 12px;
    border: 1px solid #dcdee5;
    border-top: none;
    border-radius: 0 0 2px 2px;
    background: #ffffff;
    :deep(.bk-pagination-list.is-last) {
      margin-left: auto;
    }
  }
  .record-table {
    :deep(.bk-table-body table tbody tr td) {
      .cell {
        display: inline-block;
        height: auto;
        line-height: normal;
        vertical-align: middle;
      }
      &:last-child .cell {
        // 更多操作显示
        overflow: unset;
      }
    }
  }
  // .ellipsis {
  //   overflow: hidden;
  //   text-overflow: ellipsis;
  //   white-space: nowrap;
  // }
  .multi-line-styles {
    padding: 7px 0;
    display: flex;
    justify-content: flex-start;
    align-items: center;
    width: 100%;
    height: 100%;
    min-height: 42px;
    overflow: hidden;
    white-space: normal;
    word-wrap: break-word;
    word-break: break-all;
    line-height: 21px;
  }
  .empty-action {
    margin-right: 50px;
    vertical-align: sub;
  }
  .popover-content {
    // min-width: 172px;
    font-size: 12px;
    line-height: 16px;
    color: #4d4f56;
    .itsm-sn {
      cursor: pointer;
    }
    .itsm-content {
      display: flex;
      justify-content: flex-start;
      align-items: center;
      color: #4d4f56;
      &.em {
        color: #3a84ff;
      }
      & + .itsm-title {
        margin-top: 18px;
      }
    }
    .itsm-action {
      margin-left: 10px;
      padding-left: 10px;
      display: flex;
      align-items: center;
      border-left: 1px solid #dcdee5;
      cursor: pointer;
    }
  }
</style>
