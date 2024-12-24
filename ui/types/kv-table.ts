// 接口字段设置
export interface IFiledItem {
  name: string;
  alias: string;
  primary: boolean;
  column_type: string;
  not_null: boolean;
  default_value: string | string[];
  unique: boolean;
  read_only: boolean;
  auto_increment: boolean;
  enum_value: string; // 枚举值设置内容
  selected: boolean; // 枚举值是否多选
}

// 字段设置编辑
export interface IFiledsItemEditing {
  id?: number | string;
  name: string;
  alias: string;
  column_type: string;
  default_value: string | string[];
  primary: boolean;
  not_null: boolean;
  unique: boolean;
  auto_increment: boolean;
  read_only: boolean;
  enum_value: IEnumItem[]; // 枚举值设置内容
  selected: boolean; // 枚举值是否多选
  status?: string;
  isShowBatchSet?: boolean;
}

// 托管表格数据
export interface ILocalTableItem {
  id: number;
  spec: {
    databases_name: string;
    table_name: string;
    table_memo: string;
    visible_range: number[];
    columns: IFiledItem[];
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

// 字段设置枚举类型
export interface IEnumItem {
  text: string;
  value: string;
  hasTextError?: boolean;
  hasValueError?: boolean;
}

// 托管表格新建表单编辑
export interface ILocalTableFormEditing {
  table_name: string;
  table_memo: string;
  visible_range: string[];
  columns: IFiledsItemEditing[];
}

// 托管表格新建表单
export interface ILocalTableForm {
  table_name: string;
  table_memo: string;
  visible_range: string[];
  columns: IFiledItem[];
}

// 托管表格编辑数据
export interface ILocalTableEditData {
  id: number;
  content: { [key: string]: string | string[] };
  status: string;
  isShowBatchSet?: boolean;
}
