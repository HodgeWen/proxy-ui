package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// clashEndpointConfig config.json 中 Clash API 端点的最小解析结构。
type clashEndpointConfig struct {
	Experimental *struct {
		ClashAPI *ClashAPI `json:"clash_api"`
	} `json:"experimental"`
}

// ClashClient sing-box Clash API 客户端：地址与认证取自生成的 config.json。
type ClashClient struct {
	base   string // API 基地址，如 http://127.0.0.1:19090
	secret string // Bearer 认证密钥，可空
}

// TrafficSample 实时速率样本（字节/秒）。
type TrafficSample struct {
	Up   int64 `json:"up"`   // 上行速率
	Down int64 `json:"down"` // 下行速率
}

// Connection 当前连接条目（Clash API 响应裁剪后的展示结构）。
type Connection struct {
	ID          string    `json:"id"`          // 连接 ID
	Network     string    `json:"network"`     // 传输层：tcp / udp
	Inbound     string    `json:"inbound"`     // 入站 tag（metadata.type 去掉协议前缀）
	Source      string    `json:"source"`      // 客户端来源 ip:port
	Destination string    `json:"destination"` // 目标 host 或 ip:port
	Host        string    `json:"host"`        // 目标域名（可空）
	Upload      int64     `json:"upload"`      // 该连接上行累计字节数
	Download    int64     `json:"download"`    // 该连接下行累计字节数
	Start       time.Time `json:"start"`       // 建立时间
	Chain       []string  `json:"chain"`       // 出站链（末端为最终出站）
	Rule        string    `json:"rule"`        // 命中的路由规则
}

// ConnectionsSnapshot 连接快照：连接列表、全局累计流量与内核内存占用
// （sing-box 未提供独立 /memory 端点，内存随连接快照返回）。
type ConnectionsSnapshot struct {
	UploadTotal   int64        `json:"uploadTotal"`   // 全局上行累计字节数
	DownloadTotal int64        `json:"downloadTotal"` // 全局下行累计字节数
	Memory        int64        `json:"memory"`        // 内核内存占用字节数
	Connections   []Connection `json:"connections"`   // 当前连接列表
}

// clashMetadataJSON Clash API 连接条目 metadata 的原始结构。
type clashMetadataJSON struct {
	Network         string `json:"network"`
	Type            string `json:"type"` // 入站类型/tag，如 "vless/hy-in"
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
}

// clashConnectionJSON Clash API /connections 连接条目的原始结构。
type clashConnectionJSON struct {
	ID       string            `json:"id"`
	Metadata clashMetadataJSON `json:"metadata"`
	Upload   int64             `json:"upload"`
	Download int64             `json:"download"`
	Start    time.Time         `json:"start"`
	Chain    []string          `json:"chains"`
	Rule     string            `json:"rule"`
}

// clashSnapshotJSON Clash API /connections 响应的原始结构。
type clashSnapshotJSON struct {
	DownloadTotal int64                 `json:"downloadTotal"`
	UploadTotal   int64                 `json:"uploadTotal"`
	Memory        int64                 `json:"memory"`
	Connections   []clashConnectionJSON `json:"connections"`
}

// ClashClient 返回 Clash API 客户端；地址与认证即时解析自 config.json 的
// experimental.clash_api（统计采集低频调用，无需缓存）。配置缺失时返回 nil。
func (m *Manager) ClashClient() *ClashClient {
	data, err := os.ReadFile(m.ConfigPath())
	if err != nil {
		return nil
	}
	var cfg clashEndpointConfig
	if json.Unmarshal(data, &cfg) != nil || cfg.Experimental == nil || cfg.Experimental.ClashAPI == nil {
		return nil
	}
	return &ClashClient{
		base:   "http://" + cfg.Experimental.ClashAPI.ExternalController,
		secret: cfg.Experimental.ClashAPI.Secret,
	}
}

// Traffic 读取一个实时速率样本。Clash API /traffic 为每秒一行的分块流，
// 此处读取一行样本后即断开；首行最长等待 trafficFirstLine。
func (c *ClashClient) Traffic(ctx context.Context) (TrafficSample, error) {
	ctx, cancel := context.WithTimeout(ctx, trafficFirstLine)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/traffic", nil)
	if err != nil {
		return TrafficSample{}, err
	}
	c.authorize(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TrafficSample{}, fmt.Errorf("Clash API 不可达: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TrafficSample{}, fmt.Errorf("Clash API /traffic 响应 %d", resp.StatusCode)
	}
	var sample TrafficSample
	if err := json.NewDecoder(resp.Body).Decode(&sample); err != nil {
		return TrafficSample{}, fmt.Errorf("解析 /traffic 响应失败: %w", err)
	}
	return sample, nil
}

// Connections 拉取连接快照（含全局累计流量与内核内存占用）。
func (c *ClashClient) Connections(ctx context.Context) (*ConnectionsSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/connections", nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Clash API 不可达: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Clash API /connections 响应 %d", resp.StatusCode)
	}
	var raw clashSnapshotJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析 /connections 响应失败: %w", err)
	}
	snap := &ConnectionsSnapshot{
		UploadTotal:   raw.UploadTotal,
		DownloadTotal: raw.DownloadTotal,
		Memory:        raw.Memory,
		Connections:   make([]Connection, 0, len(raw.Connections)),
	}
	for _, conn := range raw.Connections {
		snap.Connections = append(snap.Connections, Connection{
			ID:          conn.ID,
			Network:     conn.Metadata.Network,
			Inbound:     inboundTagOf(conn.Metadata.Type),
			Source:      conn.Metadata.SourceIP + ":" + conn.Metadata.SourcePort,
			Destination: destinationOf(conn.Metadata),
			Host:        conn.Metadata.Host,
			Upload:      conn.Upload,
			Download:    conn.Download,
			Start:       conn.Start,
			Chain:       conn.Chain,
			Rule:        conn.Rule,
		})
	}
	return snap, nil
}

// authorize 附加 Bearer 认证（secret 非空时）。
func (c *ClashClient) authorize(req *http.Request) {
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
}

// inboundTagOf 从 metadata.type（"协议/tag" 或纯协议名）提取入站 tag。
func inboundTagOf(metadataType string) string {
	if _, tag, found := strings.Cut(metadataType, "/"); found {
		return tag
	}
	return ""
}

// destinationOf 组合连接目标：优先域名，否则 ip:port。
func destinationOf(meta clashMetadataJSON) string {
	if meta.Host != "" {
		return meta.Host + ":" + meta.DestinationPort
	}
	return meta.DestinationIP + ":" + meta.DestinationPort
}
