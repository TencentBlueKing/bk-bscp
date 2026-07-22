import { defineStore } from 'pinia';
import { ref } from 'vue';
import http from '../request';

interface IUserInfo {
  avatar_url: string;
  username: string;
  tenant_id: string;
  time_zone: string;
  tenant_name: string;
}

export default defineStore('user', () => {
  const userInfo = ref<IUserInfo>({
    avatar_url: '',
    username: '',
    tenant_id: '',
    tenant_name: '',
    // 用户时区，IANA 格式，
    time_zone: 'Asia/Shanghai',
  });

  const getUserInfo = async () => {
    const res =  await http.get('/auth/user/info');
    userInfo.value = {
      ...res.data,
      // 用户时区，兜底使用 Asia/Shanghai
      time_zone: res.data.time_zone || 'Asia/Shanghai',
    };
    return userInfo.value;
  };

  return { userInfo, getUserInfo };
});
