<template>
  <div class="record-date-picker">
    <bk-date-picker
      ref="datePickerRef"
      style="width: 300px"
      type="datetimerange"
      append-to-body
      :disabled-date="disabledDate"
      :model-value="defaultValue"
      :shortcuts="shortcutsRange"
      :editable="false"
      :clearable="false"
      :open="open"
      @change="change"
      @click="open = !open">
      <template #confirm>
        <div>
          <bk-button class="primary-button" theme="primary" @click="handleChange"> {{ $t('确定') }} </bk-button>
        </div>
      </template>
    </bk-date-picker>
    <bk-button class="refresh-btn" @click="init">
      <right-turn-line class="icon" />
    </bk-button>
  </div>
</template>

<script setup lang="ts">
  import { onMounted, ref } from 'vue';
  import { useRoute, useRouter } from 'vue-router';
  import dayjs from 'dayjs';
  import { RightTurnLine } from 'bkui-vue/lib/icon';
  import { useI18n } from 'vue-i18n';

  const emits = defineEmits(['changeTime']);

  const route = useRoute();
  const router = useRouter();
  const { t } = useI18n();

  const shortcutsRange = ref([
    {
      text: t('近num天', { day: 7 }),
      value() {
        const end = dayjs().toDate();
        const start = dayjs().subtract(7, 'day').toDate();
        return [start, end];
      },
    },
    {
      text: t('近num天', { day: 15 }),
      value() {
        const end = dayjs().toDate();
        const start = dayjs().subtract(15, 'day').toDate();
        return [start, end];
      },
    },
    {
      text: t('近num天', { day: 30 }),
      value() {
        const end = dayjs().toDate();
        const start = dayjs().subtract(30, 'day').toDate();
        return [start, end];
      },
    },
  ]);
  const open = ref(false);
  const datePickerRef = ref(null);
  const defaultValue = ref<string[]>([]);

  onMounted(() => {
    init();
  });

  const init = () => {
    // 获取url中的时间
    // const hasQueryTime = ['start_time', 'end_time'].every(
    //   (key) => key in route.query && route.query[key]?.length === 19,
    // );
    let defaultTimeRange = [
      dayjs().subtract(7, 'day').format('YYYY-MM-DD HH:mm:ss'),
      dayjs().format('YYYY-MM-DD HH:mm:ss'),
    ];
    // 有limit参数时，不自动选择最近7天时间
    if (route.query.limit) {
      defaultTimeRange = [];
    }
    // url时间校验且用作入参
    // if (hasQueryTime) {
    //   let startTime = dayjs(String(route.query.start_time));
    //   let endTime = dayjs(String(route.query.end_time));
    //   // 验证时间格式且在当前时间以前
    //   const isValidTime = startTime.isValid() && endTime.isValid() && startTime.isBefore() && endTime.isBefore();
    //   if (isValidTime) {
    //     if (startTime.isAfter(endTime)) [startTime, endTime] = [endTime, startTime];
    //     defaultValue.value = [startTime.format('YYYY-MM-DD HH:mm:ss'), endTime.format('YYYY-MM-DD HH:mm:ss')];
    //   } else {
    //     defaultValue.value = defaultTimeRange;
    //   }
    // } else {
    //   defaultValue.value = defaultTimeRange;
    // }
    defaultValue.value = defaultTimeRange;
    // setUrlParams();
    emits('changeTime', defaultValue.value);
  };

  const change = (date: []) => {
    defaultValue.value = date;
    setUrlParams();
  };

  const setUrlParams = () => {
    const query = { ...route.query };
    // 重新选择时间后不再精确查询
    if (query.limit || query.id) {
      delete query.limit;
      delete query.id;
    }
    router.replace({
      query: {
        ...query,
        // start_time: defaultValue.value[0],
        // end_time: defaultValue.value[1],
      },
    });
  };

  const disabledDate = (date: Date) => date && date.valueOf() > Date.now() - 86400;

  const handleChange = () => {
    emits('changeTime', defaultValue.value);
    open.value = false;
  };
</script>

<style lang="scss" scoped>
  .record-date-picker {
    display: flex;
    align-items: center;
    .primary-button {
      margin-right: 4px;
      height: 26px;
    }
    .refresh-btn {
      margin-left: 8px;
      width: 32px;
      height: 32px;
      .icon {
        font-size: 16px;
      }
    }
  }
</style>
