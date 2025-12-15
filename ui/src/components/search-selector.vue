<template>
  <bk-search-select
    v-model="searchValue"
    :data="data"
    :get-menu-list="getMenuList"
    value-behavior="need-key"
    unique-select
    :placeholder="placeholder"
    @update:model-value="handleChange"
    @paste="handlePaste" />
</template>
<script setup lang="ts">
  import { ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { getUserList } from '../api';
  import { ITenantUser } from '../../types/index';
  import useGlobalStore from '../store/global';
  import { storeToRefs } from 'pinia';

  const { spaceFeatureFlags } = storeToRefs(useGlobalStore());
  const { t } = useI18n();

  interface ISearchField {
    field: string;
    label: string;
    children?: {
      name: string;
      id: string;
    }[];
  }

  interface ISearchMenuItem {
    id: string;
    name: string;
    children?: any[];
    placeholder?: string;
    async?: boolean;
    multiple?: boolean;
  }

  interface ISearchItem {
    id: string;
    name: string;
    values: {
      id: string;
      name: string;
    }[];
  }

  const props = defineProps<{
    searchField: ISearchField[];
    userField?: string[];
    placeholder: string;
  }>();
  const emits = defineEmits(['search']);

  const data = ref<ISearchMenuItem[]>([]);
  const searchValue = ref<ISearchItem[]>([]);

  watch(
    () => props.searchField,
    () => {
      data.value = props.searchField.map((item) => {
        if (props.userField?.includes(item.field)) {
          return {
            name: item.label,
            id: item.field,
            children: [],
            placeholder: t('请选择/请输入'),
            async: true,
            validate: true,
          };
        }
        return {
          name: item.label,
          id: item.field,
          children: item.children,
          placeholder: t('请选择/请输入'),
          async: false,
          multiple: item.children && item.children.length > 0,
        };
      });
    },
    { immediate: true, deep: true },
  );

  const getMenuList = async (item: ISearchMenuItem, keyword: string) => {
    // 处理searchSelect唯一选择失效
    if (!item) return data.value.filter((item) => !searchValue.value.find((value) => value.id === item.id));
    if (item.async && keyword && spaceFeatureFlags.value.ENABLE_MULTI_TENANT_MODE) {
      const res = await getUserList(keyword);
      return res.map((user: ITenantUser) => {
        return {
          id: user.bk_username,
          name: user.display_name,
        };
      });
    }
    if (item.children?.length) return item.children;
    return [];
  };

  const handleChange = (val: ISearchItem[]) => {
    searchValue.value = val;

    // 多选模式下需要已数组格式搜索
    const searchQuery = val.reduce<Record<string, string | string[]>>((acc, item) => {
      const target = data.value.find((filter) => filter.id === item.id);
      const ids = item.values.map((v) => v.id);

      acc[item.id] = target?.multiple ? ids : ids.join(',');
      return acc;
    }, {});

    emits('search', searchQuery);
  };

  const handlePaste = (e: ClipboardEvent) => {
    const clipboardData = e.clipboardData;
    if (!clipboardData) return;
    const isText = clipboardData.types.includes('text/plain');
    if (!isText) {
      // 只允许文本粘贴
      e.preventDefault();
    }
  };

  defineExpose({
    clear: () => {
      searchValue.value = [];
    },
    clearCreator: () => {
      const creatorIndex = searchValue.value.findIndex((item) => item.id === 'creator');
      if (creatorIndex !== -1) {
        searchValue.value.splice(creatorIndex, 1);
      }
    },
  });
</script>

<style scoped lang="scss"></style>
