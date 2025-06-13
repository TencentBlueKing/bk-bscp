<template>
  <bk-search-select
    :model-value="searchValue"
    :data="data"
    :get-menu-list="getMenuList"
    value-behavior="need-key"
    unique-select
    :placeholder="placeholder"
    @update:model-value="handleChange" />
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
    searchFiled: ISearchField[];
    userFiled: string[];
    placeholder: string;
  }>();
  const emits = defineEmits(['search']);

  const data = ref<ISearchMenuItem[]>([]);
  const searchValue = ref<ISearchItem[]>([]);

  onBeforeMount(() => {
    data.value = props.searchFiled.map((item) => {
      if (props.userFiled.includes(item.field)) {
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
    if (!item) return data.value;
    if (item.async && keyword && spaceFeatureFlags.value.ENABLE_MULTI_TENANT_MODE) {
      const res = await getUserList(keyword);
      return res.data.map((user: ITenantUser) => {
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

  defineExpose({
    clear: () => {
      searchValue.value = [];
    },
  });
</script>

<style scoped lang="scss"></style>
