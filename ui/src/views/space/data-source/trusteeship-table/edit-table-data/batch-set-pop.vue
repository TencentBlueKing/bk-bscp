<template>
  <bk-popover ext-cls="popover-wrap" theme="light" trigger="manual" placement="bottom" :is-show="isShow">
    <edit-line class="edit-line" @click="isShow = true" />
    <template #content>
      <div class="pop-wrap" v-click-outside="() => (isShow = false)">
        <div class="pop-content">
          <div class="pop-title">{{ $t('批量设置字段值') }}</div>
          <bk-select
            v-if="type === 'enum'"
            class="charset-select"
            v-model="localVal"
            auto-focus
            :multiple="isMultiple"
            :filterable="false">
            <bk-option v-for="(enumItem, i) in enumValue" :id="enumItem.value" :key="i" :name="enumItem.text" />
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
  import { ref } from 'vue';
  import { EditLine } from 'bkui-vue/lib/icon';

  defineProps<{
    isMultiple: boolean;
    type: string;
    enumValue: { text: string; value: string }[];
  }>();

  const emits = defineEmits(['update:isShow', 'confirm']);

  const isShow = ref(false);
  const localVal = ref('');

  const handleConfirm = () => {
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
      .bk-input,
      .charset-select {
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
</style>
