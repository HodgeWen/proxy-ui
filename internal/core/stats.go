package core

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/s-ui/s-ui/internal/db"
	"github.com/s-ui/s-ui/internal/statsproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// StatsClient polls V2Ray API for traffic stats and persists deltas to DB.
type StatsClient struct {
	addr    string
	conn    *grpc.ClientConn
	client  statsproto.StatsServiceClient
	lastSeen map[string]int64
	mu      sync.Mutex
}

// NewStatsClient creates a StatsClient for the given gRPC address (e.g. "127.0.0.1:8080").
func NewStatsClient(addr string) *StatsClient {
	return &StatsClient{
		addr:    addr,
		lastSeen: make(map[string]int64),
	}
}

// FetchAndPersist connects to the V2Ray API, fetches stats, computes deltas, and persists to DB.
// On gRPC connect failure, logs and returns without error (stats stay 0).
func (c *StatsClient) FetchAndPersist(ctx context.Context) error {
	if c.conn == nil {
		conn, err := grpc.NewClient(c.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("[stats] gRPC connect %s: %v", c.addr, err)
			return nil
		}
		c.conn = conn
		c.client = statsproto.NewStatsServiceClient(conn)
	}

	resp, err := c.client.QueryStats(ctx, &statsproto.QueryStatsRequest{Reset_: false})
	if err != nil {
		log.Printf("[stats] QueryStats: %v", err)
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, s := range resp.Stat {
		if s.Name == "" {
			continue
		}
		last := c.lastSeen[s.Name]
		delta := s.Value - last
		if delta < 0 {
			delta = s.Value
		}
		c.lastSeen[s.Name] = s.Value

		if delta == 0 {
			continue
		}

		if strings.HasPrefix(s.Name, "inbound>>>") {
			c.applyInboundStat(s.Name, delta)
		} else if strings.HasPrefix(s.Name, "user>>>") {
			c.applyUserStat(s.Name, delta)
		}
	}
	return nil
}

func (c *StatsClient) applyInboundStat(name string, delta int64) {
	// inbound>>>{tag}>>>traffic>>>uplink|downlink
	parts := strings.SplitN(name, ">>>", 4)
	if len(parts) != 4 || parts[2] != "traffic" {
		return
	}
	tag := parts[1]
	dir := parts[3]
	in, err := db.GetInboundByTag(tag)
	if err != nil {
		return
	}
	switch dir {
	case "uplink":
		in.TrafficUplink += delta
	case "downlink":
		in.TrafficDownlink += delta
	default:
		return
	}
	_ = db.UpdateInbound(in)
}

func (c *StatsClient) applyUserStat(name string, delta int64) {
	// user>>>{name}>>>traffic>>>uplink|downlink
	parts := strings.SplitN(name, ">>>", 4)
	if len(parts) != 4 || parts[2] != "traffic" {
		return
	}
	userName := parts[1]
	dir := parts[3]
	u, err := db.GetUserByName(userName)
	if err != nil {
		return
	}
	switch dir {
	case "uplink":
		u.TrafficUplink += delta
	case "downlink":
		u.TrafficDownlink += delta
	default:
		return
	}
	u.TrafficUsed = u.TrafficUplink + u.TrafficDownlink
	_ = db.UpdateUser(u)
}

// Close closes the gRPC connection.
func (c *StatsClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
