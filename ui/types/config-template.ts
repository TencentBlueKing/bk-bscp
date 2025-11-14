export interface IProcessTreeNode {
  name: string;
  id: number;
  type: string;
  children?: IProcessTreeNode[];
}
