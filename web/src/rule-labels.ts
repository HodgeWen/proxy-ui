/** 路由规则展示标签：规则列表的匹配条件与编辑弹窗的一句话预览共用的匹配方式中文短名 */
export function matchMethodLabel(method: string): string {
  switch (method) {
    case 'exact':
      return '精确'
    case 'suffix':
      return '后缀'
    case 'keyword':
      return '关键字'
    case 'regex':
      return '正则'
    case 'cidr':
      return '网段'
    default:
      return method
  }
}
