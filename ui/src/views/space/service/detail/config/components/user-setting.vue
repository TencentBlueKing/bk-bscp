<template>
  <div class="user-settings-label">{{ t('权限设置') }}</div>
  <div class="user-settings-wrap">
    <div class="user-content">
      <bk-form-item :label="$t('文件权限')" property="privilege" required>
        <permissionInputPicker v-model="localVal.privilege" :is-batch-edit="props.isBatchEdit" />
      </bk-form-item>
      <div class="user-settings">
        <bk-form-item :label="$t('用户')" property="user" :required="true">
          <customSelector
            :value="localVal.user"
            :list="userList"
            :input-width="114"
            :placeholder="isBatchEdit ? t('保持不变') : t('请输入')"
            @change="localVal.user = $event"
            @select="handleSelectUserOrGroup('user', $event)">
            <template #item="{ item }">
              <div class="option-item">
                <span>{{ item.name }}</span>
                <span
                  v-if="!item.read_only"
                  class="bk-bscp-icon icon-close-line close"
                  @click.stop="handleDeleteUserOrGroup('user', item.id)" />
              </div>
            </template>
          </customSelector>
        </bk-form-item>
        <bk-form-item :label="'UID'" property="uid" :required="true">
          <bk-input
            v-model="localVal.uid"
            :disabled="selectUser?.read_only"
            :placeholder="isBatchEdit ? t('保持不变') : t('请输入')" />
        </bk-form-item>
        <bk-form-item :label="$t('用户组')" property="user_group" :required="true">
          <customSelector
            :value="localVal.user_group"
            :list="userGroupList"
            :input-width="114"
            :placeholder="isBatchEdit ? t('保持不变') : t('请输入')"
            @change="localVal.user_group = $event"
            @select="handleSelectUserOrGroup('group', $event)">
            <template #item="{ item }">
              <div class="option-item">
                <span>{{ item.name }}</span>
                <span
                  v-if="!item.read_only"
                  class="bk-bscp-icon icon-close-line close"
                  @click.stop="handleDeleteUserOrGroup('group', item.id)" />
              </div>
            </template>
          </customSelector>
        </bk-form-item>
        <bk-form-item :label="'GID'" property="gid" :required="true">
          <bk-input
            v-model="localVal.gid"
            :disabled="selectUserGroup?.read_only"
            :placeholder="isBatchEdit ? t('保持不变') : t('请输入')" />
        </bk-form-item>
      </div>
      <p v-if="isShowTips" class="tips">
        {{ t('若需在') }}<span>{{ $t('容器') }} </span>{{ t('中拉取配置文件并设置权限，') }}
        <span>{{ t('请配置 UID 和 GID。') }} </span><br />
        {{ t('因为设置文件权限操作不是在业务容器中执行，而是在 Sidecar 容器中执行，') }}
        {{
          t('因此需要在 Sidecar容器中创建相应的用户（UID）、用户组（GID）。如果无需使用容器客户端可不配置 UID 和 GID。')
        }}
      </p>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, computed, onMounted, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    getUserPrivileges,
    getUserGroupPrivileges,
    deleteUserPrivilege,
    deleteUserGroupPrivilege,
  } from '../../../../../../api/config';
  import { IUserPrivilege, IConfigPrivilegeForm } from '../../../../../../../types/config';
  import permissionInputPicker from '../../../../../../components/permission-input-picker.vue';
  import customSelector from '../../../../../../components/custom-selector.vue';

  const { t } = useI18n();

  const props = defineProps<{
    bkBizId: string;
    id: number; // 服务ID或者模板空间ID
    isTpl?: boolean; // 是否为模板配置文件
    isBatchEdit?: boolean; // 是否为批量修改
    form: IConfigPrivilegeForm;
  }>();

  const emits = defineEmits(['change']);

  const localVal = ref(props.form);
  const userList = ref<IUserPrivilege[]>([]);
  const userGroupList = ref<IUserPrivilege[]>([]);

  watch(
    () => localVal.value,
    () => {
      emits('change', localVal.value);
    },
    { deep: true },
  );

  onMounted(async () => {
    await handleGetPrivilegesList();
    if (!props.form.uid || !props.form.gid) {
      localVal.value.uid = selectUser.value!.pid;
      localVal.value.gid = selectUserGroup.value!.pid;
    }
  });

  const selectUser = computed(() => userList.value.find((item) => item.name === localVal.value.user));
  const selectUserGroup = computed(() => userGroupList.value.find((item) => item.name === localVal.value.user_group));

  const isShowTips = ref(false);

  // 获取用户和用户组列表
  const handleGetPrivilegesList = async () => {
    try {
      const userGroupListRes = await getUserGroupPrivileges(props.bkBizId, props.id, { all: true, start: 0 });
      const userListRes = await getUserPrivileges(props.bkBizId, props.id, { all: true, start: 0 });
      userGroupList.value = userGroupListRes.data.details;
      userList.value = userListRes.data.details;
    } catch (error) {
      console.error(error);
    }
  };

  // 选择用户或用户组
  const handleSelectUserOrGroup = (type: 'user' | 'group', val: IUserPrivilege) => {
    if (type === 'user') {
      localVal.value.user = val.name;
      localVal.value.uid = val.pid;
    } else {
      localVal.value.user_group = val.name;
      localVal.value.gid = val.pid;
    }
    isShowTips.value = true;
  };

  const handleDeleteUserOrGroup = (type: 'user' | 'group', id: number) => {
    if (type === 'user') {
      deleteUserPrivilege(props.bkBizId, props.id, id);
    } else {
      deleteUserGroupPrivilege(props.bkBizId, props.id, id);
    }
    handleGetPrivilegesList();
  };
</script>

<style scoped lang="scss">
  .user-settings-label {
    font-size: 12px;
    color: #63656e;
    margin-bottom: 6px;
  }
  .user-settings-wrap {
    padding: 12px 16px 16px 16px;
    background: #f5f7fa;
    border-radius: 2px;
    .user-content {
      :deep(.bk-form-item) {
        margin-bottom: 16px;
        .bk-form-error {
          position: inherit;
        }
      }
    }
    .user-settings {
      display: flex;
      justify-content: space-between;
      :deep(.bk-input) {
        width: 114px;
      }
      :deep(.bk-form-item) {
        margin-bottom: 0;
        .bk-form-error {
          position: inherit;
        }
      }
    }
  }
  :deep(.permission-input-picker) {
    width: 252px;
  }

  .tips {
    color: #979ba5;
    font-size: 12px;
    line-height: 20px;
    margin: 8px 0 0 0;
    span {
      color: #ff9c01;
    }
  }

  .option-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    .bk-bscp-icon:hover {
      color: #3a84ff;
    }
  }
</style>

<style></style>
