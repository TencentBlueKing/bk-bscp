<template>
  <section class="single-line-kv-diff">
    <div ref="containerRef" class="config-diff-list">
      <div
        v-for="item in configList"
        :class="['config-info-item', { selected: props.selectedId === item.id }]"
        :key="item.id">
        <div :class="['diff-header', item.diffType]">
          <bk-overflow-title class="config-name" type="tips">
            {{ item.name }}
          </bk-overflow-title>
        </div>
        <div class="info-content">
          <div class="version-content">
            <div class="content-box">
              <div v-if="item.is_secret" class="secret-content">
                <template v-if="!item.secret_hidden">
                  <span>{{ item.isCipherShowValue ? '********' : item.current.content }}</span>
                  <div class="actions">
                    <Unvisible
                      v-if="item.isCipherShowValue"
                      class="view-icon"
                      @click="item.isCipherShowValue = false" />
                    <Eye v-else class="view-icon" @click="item.isCipherShowValue = true" />
                  </div>
                </template>
                <span v-else class="un-view-value">{{ $t('敏感数据不可见，无法查看实际内容') }}</span>
              </div>
              <span v-else>{{ item.current.content }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script lang="ts" setup>
  import { ref, watch, onMounted } from 'vue';
  import { ISingleLineKVDIffItem } from '../../../../../../../types/service';
  import { Unvisible, Eye } from 'bkui-vue/lib/icon';

  const props = defineProps<{
    configs: ISingleLineKVDIffItem[];
    selectedId?: number;
  }>();

  const containerRef = ref();
  const configList = ref<ISingleLineKVDIffItem[]>(props.configs);

  watch(
    [() => props.configs, () => props.selectedId],
    () => {
      setScrollTop();
    },
    {
      flush: 'post',
    },
  );

  watch(
    () => props.configs,
    () => {
      configList.value = props.configs.map((item) => {
        return {
          ...item,
          isCipherShowValue: true,
        };
      });
    },
    { immediate: true },
  );

  onMounted(() => {
    setScrollTop();
  });

  const setScrollTop = () => {
    const selectedEl = containerRef.value.querySelector('.config-info-item.selected');
    if (selectedEl) {
      containerRef.value.scrollTo(0, selectedEl.offsetTop);
    }
  };
</script>

<style scoped lang="scss">
  .single-line-kv-diff {
    position: relative;
    height: 100%;
    background: #f5f7fa;
  }
  .config-diff-list {
    height: 100%;
    overflow: auto;
  }
  .config-info-item {
    margin-bottom: 12px;
    &.selected {
      .info-content {
        background: #f0f1f5;
      }
      .content-box {
        border-color: #3a84ff;
      }
    }
  }
  .diff-header {
    padding: 2px 16px;
    background: #eaebf0;
    &.modify {
      background: #fff1db;
      .config-name {
        color: #fe9c00;
      }
    }
    &.add {
      background: #edf4ff;
      .config-name {
        color: #3a84ff;
      }
    }
    &.delete {
      background: #feebea;
      .config-name {
        color: #ea3536;
      }
    }
    .config-name {
      line-height: 20px;
      font-size: 12px;
      color: #63656e;
      max-width: 48%;
    }
  }
  .info-content {
    display: flex;
    align-items: flex-start;
  }
  .version-content {
    padding: 8px 16px 12px;
    width: 100%;
    height: 100%;
  }
  .content-box {
    display: flex;
    align-items: center;
    // width: 435px;
    padding: 0 15px;
    min-height: 52px;
    line-height: 20px;
    background: #ffffff;
    border: 1px solid #c4c6cc;
    border-radius: 2px;
    font-size: 12px;
    & > span {
      word-wrap: break-word;
      word-break: break-all;
    }
    .secret-content {
      width: 100%;
      display: flex;
      align-items: center;
      justify-content: space-between;
      & > span {
        word-wrap: break-word;
        word-break: break-all;
      }
      .view-icon {
        cursor: pointer;
        font-size: 14px;
        color: #979ba5;
        &:hover {
          color: #3a84ff;
        }
      }
      .un-view-value {
        color: #c4c6cc;
      }
    }
  }
</style>
