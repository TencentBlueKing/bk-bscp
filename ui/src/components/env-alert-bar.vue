<template>
  <div
    :class="`env-alert-bar env-alert-bar--${envType}`"
    :style="{
      backgroundColor: ENV_TYPE_CONFIG[envType]?.bgColor || '#F5F7FA',
    }">
    <div
      class="env-tag"
      :style="{
        backgroundColor: ENV_TYPE_CONFIG[envType]?.bgColor || '#F5F7FA',
        color: ENV_TYPE_CONFIG[envType]?.textColor || '#63656E',
      }">
      <i
       :class="`bk-bscp-icon ${ENV_TYPE_CONFIG[envType].iconClass} env-icon`"
       :style="{ color: ENV_TYPE_CONFIG[envType]?.iconColor || '#979BA5' }"></i>
      <span class="env-name">{{ envName }}</span>
    </div>
    <div class="env-message">{{ ENV_TYPE_CONFIG[envType].tip }}</div>
    <env-selector
      class="env-alert-switch"
      :model-value="modelValue"
      @change="handleChange">
      <template #trigger>
        <bk-button class="env-switch-btn" size="small">
          <Transfer class="switch-icon"/>
          {{ t('切换环境') }}
        </bk-button>
      </template>
    </env-selector>
  </div>
</template>

<script setup lang="ts">
  import { ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import EnvSelector from './env-selector.vue';
  import { EnvType, IEnvItem } from '../../types/env';
  import { ENV_TYPE_CONFIG } from '../constants/env';
  import { Transfer } from 'bkui-vue/lib/icon';

  const { t } = useI18n();

  withDefaults(
    defineProps<{
      modelValue?: string;
    }>(),
    {
      modelValue: '',
    },
  );

  // eslint-disable-next-line func-call-spacing
  const emit = defineEmits<{
    (e: 'change', env: IEnvItem): void;
  }>();

  const envType = ref(EnvType.PRODUCTION);
  const envName = ref('');
  const handleChange = (env: IEnvItem) => {
    envType.value = env.spec.type;
    envName.value = env.spec.name || '';
    emit('change', env);
  };
</script>

<style lang="scss" scoped>
  .env-alert-bar {
    display: flex;
    justify-content: center;
    align-items: center;
    height: 36px;
    padding: 6px 0;
    box-sizing: border-box;
    border-bottom: 2px solid transparent;
    color: #313238;

    .env-tag {
      display: inline-flex;
      align-items: center;
      margin-right: 24px;
      .env-icon {
        font-size: 20px;
        margin-right: 6px;
      }
      .env-name {
        font-weight: 700;
      }
    }

    .env-message {
      min-width: 432px;
      margin-right: 31px;
      font-size: 12px;
      font-weight: 400;
    }

    .env-switch-btn {
      font-size: 12px;
      font-weight: 400;
      padding: 0 6px;
      background: transparent;
      .switch-icon {
        font-size: 16px;
        margin-right: 4px;
      }
    }

    &--prod {
      border-image:
      linear-gradient(90deg, #FFF0F0 0%, #FFF0F0 20%, #EA3636 40%, #EA3636 60%, #FFF0F0 80%, #FFF0F0 100%) 1;
      .env-switch-btn {
        color: #E71818;
        border-color: #FF5656;
        .switch-icon {
            color: #EA3636;
        }
      }
    }
    &--staging {
      border-image:
      linear-gradient(90deg, #FDF4E8 0%, #FDF4E8 20%, #F59500 40%, #F59500 60%, #FDF4E8 80%, #FDF4E8 100%) 1;
      .env-switch-btn {
        color: #CC8800;
        border-color: #F8B64F;
      }
    }
    &--test {
      border-image:
       linear-gradient(90deg, #F0F5FF 0%, #F0F5FF 20%, #699DF4 40%, #699DF4 60%, #F0F5FF 80%, #F0F5FF 100%) 1;
      .env-switch-btn {
        color: #3a84ff;
        border-color: #699DF4;
        .switch-icon {
            color: #699DF4;
        }
      }
    }
    &--dev {
      border-image:
      linear-gradient(90deg, #E8F5E8 0%, #E8F5E8 20%, #2DCB56 40%, #2DCB56 60%, #E8F5E8 80%, #E8F5E8 100%) 1;
      .env-switch-btn {
        color: #299E56;
        border-color: #65C389;
        .switch-icon {
            color: #2CAF5E;
        }
      }
    }
  }
</style>
