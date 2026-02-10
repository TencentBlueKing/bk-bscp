import dayjs from 'dayjs';

// 配置文件编辑参数
export function getConfigTemplateEditParams() {
  return {
    name: '',
    memo: '',
    file_name: '',
    file_path: '',
    file_type: 'text',
    file_mode: 'unix',
    user: 'root',
    user_group: 'root',
    privilege: '644',
    charset: 'UTF-8',
    fileAP: '',
    highlight_style: 'python',
    revision_name: `v${dayjs().format('YYYYMMDDHHmmss')}`,
  };
}
