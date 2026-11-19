# 简化登录页为品牌单面板

## 补丁内容

将登录页从左右分栏、多信息块的结构收缩为单一登录面板，只保留品牌标题、简洁图形标识、背景氛围层和必要的登录表单内容，避免页面信息密度过高。

表单区域继续使用 HeroUI 的 `Card`、`Form`、`TextField`、`Input`、`Checkbox`、`Button` 组织，保留错误反馈、记住登录与初始化提示，同时让 `S-UI` 品牌识别更直接。

## 影响范围

- 修改文件: `/home/whj/codes/s-ui/web/src/pages/Login.tsx`
- 新增文件: `/home/whj/codes/s-ui/.agent-context/plan-3/patch-2.md`
