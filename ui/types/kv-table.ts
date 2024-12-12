export interface ITableFiledItem {
  name: string;
  alias: string;
  length: number;
  primary: boolean;
  column_type: string;
  nullable: boolean;
  default_value: string;
  unique: boolean;
  read_only: boolean;
}

export interface ILocalTableItem {
  id: number;
  spec: {
    databases_name: string;
    table_name: string;
    table_memo: string;
    visible_range: number[];
    columns: ITableFiledItem[];
  };
  attachment: {
    biz_id: number;
    data_source_info_id: number;
  }; // 表的附加信息
  revision: {
    creator: string;
    create_at: string;
  };
}

// 字段设置列表项
export interface IFiledsItem {
  id: number;
  name: string;
  alias: string;
  column_type: string;
  default_value: string;
  primary: boolean;
  nullable: boolean;
  unique: boolean;
  auto_increment: boolean;
  read_only: boolean;
  enumList?: IEnumItem[];
  enumType?: string;
  isShowSettingEnumPopover?: boolean;
  status?: string;
}

// 字段设置枚举类型
export interface IEnumItem {
  text: string;
  value: string;
  hasTextError?: boolean;
  hasValueError?: boolean;
}
