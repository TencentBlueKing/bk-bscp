import { defineStore } from 'pinia';
import { ref } from 'vue';
import http from '../request';

interface IUserInfo {
  avatar_url: string;
  username: string;
  tenant_id: string;
}

export default defineStore('user', () => {
  const userInfo = ref<IUserInfo>({
    avatar_url: '',
    username: '',
    tenant_id: '',
  });

  const getUserInfo = async () => {
    const res =  await http.get('/auth/user/info');
    userInfo.value = res.data as IUserInfo;
    return userInfo.value;
  };

  return { userInfo, getUserInfo };
});
