import { IFileConfigContentSummary } from './config';
import { IVariableEditParams } from './variable';

export interface IServiceEditForm {
  name: string;
  alias: string;
  config_type: string;
  memo: string;
  data_type?: string;
  is_approve: boolean;
  approver: string;
  approve_type: string;
  // encryptionSwtich: boolean;
  // encryptionKey: string;
}

export interface ISingleLineKVDIffItem {
  id: number;
  name: string;
  diffType: string;
  is_secret: boolean;
  secret_hidden: boolean;
  base: {
    content: string;
  };
  current: {
    content: string;
  };
  isCipherShowValue?: boolean;
}

// 版本下的脚本配置
export interface IDiffDetail {
  contentType: 'file' | 'text' | 'singleLineKV';
  id: number | string;
  is_secret: boolean;
  secret_hidden: boolean;
  base: {
    content: string | IFileConfigContentSummary;
    language?: string;
    variables?: IVariableEditParams[];
    permission?: {
      privilege: string;
      user: string;
      user_group: string;
    };
  };
  current: {
    content: string | IFileConfigContentSummary;
    language?: string;
    variables?: IVariableEditParams[];
    permission?: {
      privilege: string;
      user: string;
      user_group: string;
    };
  };
  singleLineKVDiff?: ISingleLineKVDIffItem[];
}

interface IPublishRecord {
  publish_time: string;
  name: string;
  scope: any;
  creator: string;
  fully_released: boolean;
  updated_at: string;
  final_approval_time: string;
}

export interface IPublishData {
  is_publishing?: boolean; // 是否有其他版本在上线
  final_approval_time: string; // 最近上线版本/驳回/通过/撤销 的时间
  version_name: string; // 最后上线的版本名称
  publish_record?: IPublishRecord[]; // 最近的上线记录
}
