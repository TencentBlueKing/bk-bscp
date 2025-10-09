import pinia from '../store/index';
import { getSpaceList } from '../api';
import useGlobalStore from '../store/global';
import useUserStore from '../store/user';

const loadSpaceList = async () => {
  const globalStore = useGlobalStore(pinia);
  const { getUserInfo } = useUserStore(pinia);
  const [spacesData, userInfo] = await Promise.all([getSpaceList(), getUserInfo()]);

  globalStore.$patch((state) => {
    state.spaceList = spacesData.items;
  });
  return userInfo;
};

// 加载全部空间列表
export default loadSpaceList;
