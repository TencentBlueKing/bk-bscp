<template>
  <bk-dialog
    class="project-form-dialog"
    v-model:is-show="localVisible"
    :title="isEdit ? t('编辑项目') : t('新增项目')"
    theme="primary"
    width="480"
    @closed="handleClosed">
    <bk-form ref="formRef" form-type="vertical" :model="formData" :rules="formRules" class="project-form">
      <bk-form-item :label="t('项目名称')" property="name" required>
        <bk-input v-model="formData.name" :placeholder="t('请输入项目名称')" />
      </bk-form-item>
      <bk-form-item :label="t('项目描述')" property="memo" required>
        <bk-input
          v-model="formData.memo"
          type="textarea"
          :placeholder="t('请输入项目描述')"
          :rows="3"
          :resize="false" />
      </bk-form-item>
    </bk-form>
    <template #footer>
      <div class="dialog-footer">
        <bk-button theme="primary" :loading="submitLoading" @click="handleSubmit">
          {{ isEdit ? t('保存') : t('确定') }}
        </bk-button>
        <bk-button @click="handleCancel">{{ t('取消') }}</bk-button>
      </div>
    </template>
  </bk-dialog>
</template>

<script setup lang="ts">
  import { ref, reactive, computed, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import Message from 'bkui-vue/lib/message';
  import { storeToRefs } from 'pinia';
  import useGlobalStore from '../../../../store/global';
  import {
    createProject as createProjectApi,
    updateProject,
  } from '../../../../api/project';
  import type { IProjectItem } from '../../../../../types/project';

  const { t } = useI18n();
  const { spaceId } = storeToRefs(useGlobalStore());

  const props = defineProps<{
    modelValue: boolean;
    editingItem: Partial<IProjectItem>;
  }>();

  // eslint-disable-next-line func-call-spacing
  const emit = defineEmits<{
    (e: 'update:modelValue', val: boolean): void;
    (e: 'success'): void;
  }>();

  const localVisible = computed({
    get: () => props.modelValue,
    set: (val: boolean) => emit('update:modelValue', val),
  });

  const isEdit = computed(() => !!props.editingItem.id);

  const formRef = ref();
  const submitLoading = ref(false);
  const formData = reactive({
    name: '',
    memo: '',
  });

  const formRules = {
    name: [
      {
        required: true,
        message: () => t('请输入项目名称'),
        trigger: 'blur',
      },
    ],
    memo: [
      {
        required: true,
        message: () => t('请输入项目描述'),
        trigger: 'blur',
      },
    ],
  };

  // 监听 editingItem 变化，填充表单数据
  watch(
    () => props.editingItem,
    (val) => {
      if (val.id) {
        Object.assign(formData, {
          name: val.spec?.name || '',
          memo: val.spec?.memo || '',
        });
      } else {
        Object.assign(formData, { name: '', memo: '' });
      }
    },
    { immediate: true },
  );

  const handleSubmit = async () => {
    try {
      await formRef.value?.validate();
      console.log(formData, ';;;');
      submitLoading.value = true;
      if (isEdit.value) {
        await updateProject(spaceId.value, props.editingItem.id! as string, formData);
        Message({ theme: 'success', message: t('编辑项目成功') });
      } else {
        await createProjectApi(spaceId.value, formData);
        Message({ theme: 'success', message: t('创建项目成功') });
      }
      localVisible.value = false;
      emit('success');
    } catch {
      // 校验失败或接口报错，不关闭弹窗
    } finally {
      submitLoading.value = false;
    }
  };

  const handleCancel = () => {
    localVisible.value = false;
  };

  const handleClosed = () => {
    Object.assign(formData, { name: '', memo: '' });
    submitLoading.value = false;
  };
</script>

<style lang="scss" scoped>
.project-form-dialog {
    .dialog-footer {
        .bk-button:first-child {
            margin-right: 8px;
        }
    }
}
</style>
