# 精简登录页会话噪音并重做 Setup 首启界面

## 补丁内容

移除登录页中无实际行为意义的“保持登录状态”勾选项，并删除底部的 Cookie 会话说明，改为更简洁的单入口表单结构，只保留管理员登录和首次部署初始化入口。

同时重做 `Setup` 页面视觉布局，将原本普通居中表单升级为带首启引导区和表单区的双栏卡片，强化品牌感、步骤感和账户要求说明，改善首屏层次与移动端呈现。

## 影响范围

- 修改文件: `/home/whj/codes/s-ui/web/src/pages/Login.tsx`
- 修改文件: `/home/whj/codes/s-ui/web/src/pages/Setup.tsx`
- 新增文件: `/home/whj/codes/s-ui/.agent-context/plan-3/patch-3.md`
