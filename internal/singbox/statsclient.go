package singbox

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
)

// errNoStatsAddr 统计服务未配置监听地址（v2ray_api 未启用）。
var errNoStatsAddr = errors.New("统计服务未启用")

// statsMethodPrefix v2ray 统计服务全名前缀。
//
// 注意：sing-box 在 experimental/v2rayapi 的 init() 里把 ServiceDesc.ServiceName 改写为
// v2ray.core.app.stats.command.StatsService（兼容 v2ray 客户端），pb 里的
// experimental.v2rayapi.StatsService 是错的，调用会得到 Unimplemented。
const statsMethodPrefix = "/v2ray.core.app.stats.command.StatsService/"

// invokeStats 调用统计服务的指定方法。
func invokeStats(ctx context.Context, conn *grpc.ClientConn, method string, req, resp any) error {
	return conn.Invoke(ctx, statsMethodPrefix+method, req, resp)
}

// userCounter 从按用户计数器名解析用户名。
//
//	user>>>alice>>>traffic>>>uplink  -> alice, true
//	inbound>>>vless-in>>>traffic>>>... -> "", false（入站维度不参与按用户统计）
func userCounter(name string) (string, bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
		return "", false
	}
	return parts[1], true
}

// isUplink 判断计数器是否为上行。
func isUplink(name string) bool { return strings.HasSuffix(name, ">>>uplink") }
