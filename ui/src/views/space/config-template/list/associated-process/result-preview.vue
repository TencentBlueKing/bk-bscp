<template>
  <div class="process-list-container">
    <div class="title" @click="isShowProcess = !isShowProcess">
      <angle-down-fill :class="['angle-icon', isShowProcess && 'expanded']" />
      {{ $t('已选') }}
      <span class="process-length">{{ process.length }}</span>
      {{ isTemplate ? $t('个模板进程') : $t('个实例进程') }}
    </div>
    <!-- 模板进程列表 -->
    <ul class="process-list" v-show="isShowProcess">
      <template v-for="(item, index) in process" :key="item.id">
        <li class="process-item">
          <div class="white-card">
            <div v-bk-overflow-tips class="white-card-left">{{ item.topoName }}</div>
            <div class="white-card-right">
              {{ item.topoParentName }}
            </div>
            <div class="close-icon-container">
              <close class="close-icon" @click="emits('remove', item, index)" />
            </div>
          </div>
        </li>
      </template>
    </ul>
  </div>
</template>

<script lang="ts" setup>
  import { ref } from 'vue';
  import { AngleDownFill, Close } from 'bkui-vue/lib/icon';
  import { IProcessPreviewItem } from '../../../../../../types/config-template';

  defineProps<{
    isTemplate: boolean; // 是否是模板进程
    process: IProcessPreviewItem[];
  }>();
  const emits = defineEmits(['remove']);

  const isShowProcess = ref(true);
</script>

<style scoped lang="scss">
  .process-list-container {
    margin-top: 15px;

    .title {
      display: flex;
      align-items: center;
      height: 30px;
      line-height: 20px;
      margin-bottom: 5px;
      color: #313238;
      font-size: 14px;
      cursor: pointer;
      transition: background-color 0.3s;

      &:hover {
        background-color: #f0f1f5;
        transition: background-color 0.3s;
      }

      .angle-icon {
        margin-right: 4px;
        color: #c4c6cc;
        transition: transform 0.3s;
        transform: rotate(90deg);
        &.expanded {
          transform: rotate(180deg);
          transition: transform 0.3s;
        }
      }

      .process-length {
        color: #3a84ff;
        font-weight: bold;
        padding: 0 4px;
      }
    }

    .process-list {
      .process-item {
        display: flex;

        &:not(:last-child) {
          margin-bottom: 4px;
        }

        .white-card {
          display: flex;
          justify-content: space-between;
          align-items: center;
          width: 100%;
          height: 32px;
          line-height: 16px;
          padding: 0 18px;
          background-color: #fff;
          border-radius: 2px;
          box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.06);

          .white-card-left {
            width: 100%;
            height: 18px;
            margin-top: 1px;
            overflow: hidden;
            white-space: nowrap;
            text-overflow: ellipsis;
          }

          .white-card-right {
            flex-shrink: 0;
            display: flex;
            align-items: center;
            margin-left: 12px;
            font-size: 12px;
            color: #979ba5;

            .gsekit-icon-parenet-node-line {
              margin-right: 4px;
              font-size: 16px;
            }
          }
        }

        .close-icon-container {
          display: flex;
          align-items: center;
          margin-left: 8px;
          .close-icon {
            font-size: 16px;
            color: #c4c6cc;
            cursor: pointer;
            &:hover {
              color: #3a84ff;
            }
          }
        }
      }
    }
  }
</style>
