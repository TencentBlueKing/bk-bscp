<template>
  <bk-popover
    :is-show="isShowPopover"
    ref="popoverRef"
    theme="light"
    trigger="manual"
    :arrow="false"
    ext-cls="custom-selector-popover"
    placement="bottom">
    <bk-input
      v-model="localVal"
      ref="inputRef"
      :style="{ width: `${inputWidth}px` }"
      class="input"
      :placeholder="placeholder"
      @click="handleClickInput"
      @enter="handleKeyInputEnter">
      <template #suffix>
        <angle-down :class="['suffix-icon', { 'show-popover': isShowPopover }]" @click="handleClickInput" />
      </template>
    </bk-input>
    <template #content>
      <div
        :style="{ width: `${inputWidth}px` }"
        class="selector-list-wrapper"
        v-click-outside="() => (isShowPopover = false)">
        <div v-for="item in list" :key="item" class="selector-option" @click.stop="handleSelectItem(item)">
          <slot name="item" :item="item"></slot>
        </div>
      </div>
    </template>
  </bk-popover>
</template>

<script lang="ts" setup>
  import { ref, onMounted, watch } from 'vue';
  import { AngleDown } from 'bkui-vue/lib/icon';

  const props = withDefaults(
    defineProps<{
      value: string;
      list: any;
      placeholder?: string;
      inputWidth?: number;
    }>(),
    {
      inputWidth: 120,
    },
  );
  const emits = defineEmits(['change', 'select']);

  const localVal = ref('');
  const isShowPopover = ref(false);
  const inputRef = ref();
  const popoverRef = ref();

  onMounted(() => {
    localVal.value = props.value;
  });

  watch(
    () => props.value,
    (val) => {
      localVal.value = val;
    },
  );

  watch(
    () => localVal.value,
    (val) => {
      emits('change', val);
    },
  );

  const handleKeyInputEnter = () => {
    inputRef.value.blur();
    isShowPopover.value = false;
  };

  const handleClickInput = () => {
    isShowPopover.value = true;
    inputRef.value.focus();
  };

  const handleSelectItem = (item: any) => {
    emits('select', item);
    isShowPopover.value = false;
  };
</script>

<style scoped lang="scss">
  .input {
    .suffix-icon {
      width: 20px;
      font-size: 20px;
      color: #979ba5;
      background-color: #fff;
      margin-right: 2px;
      &.show-popover {
        transform: rotate(180deg);
      }
    }
  }

  .selector-list-wrapper {
    padding: 4px 0;
    max-height: 200px;
    overflow: auto;
    .selector-option {
      height: 32px;
      line-height: 32px;
      padding: 0 12px;
      cursor: pointer;
      &:hover {
        background-color: #f5f7fa;
        color: #63656e;
      }
    }
  }
</style>

<style>
  .bk-popover.bk-pop2-content.custom-selector-popover {
    padding: 0;
  }
</style>
