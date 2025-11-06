<template>
  <bk-search-select
    :model-value="searchValue"
    :data="data"
    :get-menu-list="getMenuList"
    value-behavior="need-key"
    unique-select
    :placeholder="placeholder"
    @update:model-value="handleChange"
    @paste="handlePaste" />
</template>
<script setup lang="ts">
  import { ref, onBeforeMount } from 'vue';
  import { getUserList } from '../api';
  import { ITenantUser } from '../../types/index';
  import useGlobalStore from '../store/global';
  import { storeToRefs } from 'pinia';

  const { spaceFeatureFlags } = storeToRefs(useGlobalStore());

  interface ISearchField {
    field: string;
    label: string;
  }

  interface ISearchMenuItem {
    id: string;
    name: string;
    children?: any[];
    placeholder?: string;
    async?: boolean;
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
    userField: string[];
    placeholder: string;
  }>();
  const emits = defineEmits(['search']);

  const data = ref<ISearchMenuItem[]>([]);
  const searchValue = ref<ISearchItem[]>([]);

  onBeforeMount(() => {
    data.value = props.searchField.map((item) => {
      if (props.userField.includes(item.field)) {
        return {
          name: item.label,
          id: item.field,
          children: [],
          placeholder: '请选择/请输入',
          async: true,
          validate: true,
        };
      }
      return {
        name: item.label,
        id: item.field,
        children: [],
        placeholder: '请选择/请输入',
        async: false,
      };
    });
  });

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
    return [];
  };

  const handleChange = (val: ISearchItem[]) => {
    searchValue.value = val;
    const searchQuery: { [key: string]: string } = {};
    val.forEach((item) => {
      searchQuery[item.id] = item.values.map((value) => value.id).join(',');
    });
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
