package api

import (
	"context"
	"errors"
	"sync"
	"testing"

	"proxy-ui/internal/singbox"
)

// TestApplyKernelConfigSkipsCanceledContext 已取消的生命周期上下文不得再拉起内核。
func TestApplyKernelConfigSkipsCanceledContext(t *testing.T) {
	a := newInboundTestAPI(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := a.applyKernelConfig(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消 ctx 应返回 context.Canceled, got %v", err)
	}
	a.StartKernel(ctx)
	if a.core.Status().Running {
		t.Fatal("已取消 ctx 不得启动内核")
	}
}

// TestStopKernelPreventsLateStart StopKernel 与 applyKernelConfig 经 applyMu 串行：
// StopKernel 之后任何延迟到达的 StartKernel/热应用都被拒绝；并发启动与停机最终必处于已停机状态。
func TestStopKernelPreventsLateStart(t *testing.T) {
	a := newInboundTestAPI(t)
	t.Cleanup(func() { _ = a.StopKernel() })

	if err := a.StopKernel(); err != nil {
		t.Fatalf("StopKernel 失败: %v", err)
	}
	if err := a.applyKernelConfig(context.Background()); !errors.Is(err, errKernelStopped) {
		t.Fatalf("StopKernel 后 applyKernelConfig 应返回 errKernelStopped, got %v", err)
	}
	a.StartKernel(context.Background())
	if a.core.Status().Running {
		t.Fatal("StopKernel 后延迟调用 StartKernel 不得拉起内核")
	}

	// 若当前测试二进制含内核构建标签，验证并发 StartKernel 与 StopKernel 不会在停机后残留运行中内核
	if a.core.CheckConfig(singbox.MinConfig()) != nil {
		return
	}
	for range 3 {
		b := newInboundTestAPI(t)
		var wg sync.WaitGroup
		wg.Go(func() { b.StartKernel(context.Background()) })
		wg.Go(func() { _ = b.StopKernel() })
		wg.Wait()
		if b.core.Status().Running {
			_ = b.StopKernel()
			t.Fatal("并发 StartKernel 与 StopKernel 后不应残留运行中内核")
		}
	}
}
