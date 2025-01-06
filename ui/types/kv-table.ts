// 接口字段设置
export interface IFieldItem {
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
  default_value: string | string[] | undefined;
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

// 托管表格列表
export interface ILocalTableItem {
  id: number;
  spec: {
    databases_name: string;
    table_name: string;
    table_memo: string;
    visible_range: number[];
    columns: IFieldItem[];
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
  columns: IFieldItem[];
}

// 托管表格单行数据
export interface ILocalTableDataItem {
  id: number;
  spec: {
    content: { [key: string]: string | string[] };
    status: string;
  };
  attachment: {
    data_source_mapping_id: number;
  };
  revision: {
    creator: string;
    reviser: string;
    create_at: string;
    update_at: string;
  };
}

// 托管表格编辑数据
export interface ILocalTableEditData {
  id: number;
  custom_id: number;
  content: { [key: string]: string | string[] };
  status: string;
}

export interface ILocalTableEditContent {
  table_content_id: number;
  content: { [key: string]: string | string[] };
}

export const enum EDataCleanType {
  '=' = 'eq',
  '!=' = 'ne',
  '>' = 'gt',
  '>=' = 'ge',
  '<' = 'lt',
  '<=' = 'le',
  'IN' = 'in',
  'NOT IN' = 'nin',
}

export interface IDataCleanItem {
  key: string;
  op: EDataCleanType | string;
  value: string | number | string[];
}

export interface IConfigTableForm {
  managed_table_id?: number; // 托管表格id
  external_source_id?: number; // 外部数据源id
  filter_condition?: {
    labels_and?: IDataCleanItem[];
  }; // 数据清洗条件
  filter_fields?: string[]; // 过滤表格字段
}
