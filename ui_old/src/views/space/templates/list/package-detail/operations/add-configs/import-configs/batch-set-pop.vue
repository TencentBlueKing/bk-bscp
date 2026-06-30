<template>
  <bk-popover ext-cls="popover-wrap" theme="light" trigger="manual" placement="bottom" :is-show="isShow">
    <edit-line class="edit-line" @click="isShow = true" />
    <template #content>
      <div
        :class="['pop-wrap', { 'privilege-wrap': setType === 'privilege' }]"
        v-click-outside="() => (isShow = false)">
        <div class="pop-content">
          <div class="pop-title">{{ title }}</div>
          <template v-if="setType === 'privilege'">
            <bk-input
              v-model="localVal"
              style="width: 184px; margin-bottom: 16px"
              @blur="testPrivilegeInput"></bk-input>
            <span class="error-tip" style="margin-left: 10px" v-if="isShowPrivilegeError">
              {{ t('只能输入三位 0~7 数字且文件own必须有读取权限') }}
            </span>
            <div class="privilege-select-panel">
              <div v-for="(item, index) in PRIVILEGE_GROUPS" class="group-item" :key="index" :label="item">
                <div class="header">{{ item }}</div>
                <div class="checkbox-area">
                  <bk-checkbox-group
                    class="group-checkboxs"
                    :model-value="privilegeGroupsValue(localVal)[index]"
                    @change="handleSelectPrivilege(index, $event)">
                    <bk-checkbox size="small" :label="4" :disabled="index === 0">{{ t('读') }}</bk-checkbox>
                    <bk-checkbox size="small" :label="2">{{ t('写') }}</bk-checkbox>
                    <bk-checkbox size="small" :label="1">{{ t('执行') }}</bk-checkbox>
                  </bk-checkbox-group>
                </div>
              </div>
            </div>
          </template>
          <bk-select
            v-else-if="setType === 'charset'"
            class="charset-select"
            v-model="localVal"
            auto-focus
            :filterable="false"
            :clearable="false">
            <bk-option v-for="charset in charsetList" :id="charset" :key="charset" :name="charset" />
          </bk-select>
          <bk-input v-else v-model="localVal"></bk-input>
        </div>
        <div class="pop-footer">
          <div class="button">
            <bk-button theme="primary" style="margin-right: 8px; width: 80px" size="small" @click="handleConfirm">
              {{ $t('确定') }}
            </bk-button>
            <bk-button size="small" @click="handleCancel">{{ $t('取消') }}</bk-button>
          </div>
        </div>
      </div>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref, computed } from 'vue';
  import { EditLine } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';
  const { t } = useI18n();

  const props = defineProps<{
    title: string;
    setType?: string;
  }>();

  const emits = defineEmits(['update:isShow', 'confirm']);

  const PRIVILEGE_GROUPS = [t('属主（own）'), t('属组（group）'), t('其他人（other）')];
  const PRIVILEGE_VALUE_MAP = {
    0: [],
    1: [1],
    2: [2],
    3: [1, 2],
    4: [4],
    5: [1, 4],
    6: [2, 4],
    7: [1, 2, 4],
  };
  const charsetList = ['UTF-8', 'GBK'];

  const isShow = ref(false);
  const localVal = ref(props.setType === 'privilege' ? '644' : '');
  const isShowPrivilegeError = ref(false);

  const testPrivilegeInput = () => {
    const val = String(localVal.value);
    const own = parseInt(localVal.value[0], 10);
    if (/^[0-7]{3}$/.test(val) && own >= 4) {
      isShowPrivilegeError.value = false;
    } else {
      localVal.value = '644';
      isShowPrivilegeError.value = true;
    }
  };

  // 将权限数字拆分成三个分组配置
  const privilegeGroupsValue = computed(() => (privilege: string) => {
    const data: { [index: string]: number[] } = { 0: [], 1: [], 2: [] };
    if (privilege.length > 0) {
      const valArr = privilege.split('').map((i) => parseInt(i, 10));
      valArr.forEach((item, index) => {
        data[index as keyof typeof data] = PRIVILEGE_VALUE_MAP[item as keyof typeof PRIVILEGE_VALUE_MAP];
      });
    }
    return data;
  });

  // 选择文件权限
  const handleSelectPrivilege = (index: number, val: number[]) => {
    const groupsValue = { ...privilegeGroupsValue.value(localVal.value) };

    groupsValue[index] = val;
    const digits = [];
    for (let i = 0; i < 3; i++) {
      let sum = 0;
      if (groupsValue[i].length > 0) {
        sum = groupsValue[i].reduce((acc, crt) => acc + crt, 0);
      }
      digits.push(sum);
    }
    const newVal = digits.join('');
    localVal.value = newVal;
    isShowPrivilegeError.value = false;
  };

  const handleConfirm = () => {
    if (props.setType === 'privilege' && isShowPrivilegeError.value) {
      return;
    }
    emits('confirm', localVal.value);
    isShow.value = false;
  };

  const handleCancel = () => {
    localVal.value = '';
    isShow.value = false;
  };
</script>

<style scoped lang="scss">
  .edit-line {
    color: #3a84ff;
    cursor: pointer;
    text-align: right;
  }
  .pop-wrap {
    width: 240px;
    .pop-content {
      padding: 16px;
      .pop-title {
        line-height: 24px;
        font-size: 16px;
        padding-bottom: 10px;
      }
      .bk-input,.charset-select {
        width: 150px;
      }
    }

    .pop-footer {
      position: relative;
      height: 42px;
      background: #fafbfd;
      border-top: 1px solid #dcdee5;
      .button {
        position: absolute;
        right: 16px;
        top: 50%;
        transform: translateY(-50%);
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
  .error-tip {
    color: red;
  }
  .privilege-wrap {
    width: auto;
  }
</style>
