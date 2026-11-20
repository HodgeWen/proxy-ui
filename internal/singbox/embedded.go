// Package singbox 管理内置 sing-box 内核：配置生成、进程内实例生命周期与流量统计。
//
// 内核以库形式编进面板（依赖见 go.mod），不再下载外部二进制。这样能启用 v2ray_api，
// 从而按「用户」统计流量——这是共享入站下实现配额与到期的前提。
// 技术细节见 .agents/docs/SPEC-core-embedded.md。
package singbox

// EmbeddedVersion 内置内核版本（编译期常量；版本由 go.mod 里 sing-box 依赖固定）。
const EmbeddedVersion = "1.14.4"
