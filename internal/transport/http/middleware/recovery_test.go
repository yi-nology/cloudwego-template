package middleware

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	log "{{.module_name}}/internal/pkg/logger"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func setupLogger(t *testing.T) *syncBuffer {
	t.Helper()
	sb := &syncBuffer{}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(sb),
		zapcore.DebugLevel,
	)
	log.Logger = zap.New(core)
	t.Cleanup(func() { log.Logger = nil })
	return sb
}

// runRecovery 走真实中间件链：handlers[0] 是 Recovery，handlers[1] 是被测 handler。
func runRecovery(t *testing.T, requestURI string, handler app.HandlerFunc) *app.RequestContext {
	t.Helper()
	c := app.NewContext(8)
	c.Request.SetRequestURI(requestURI)
	c.Request.Header.SetMethodBytes([]byte("GET"))
	handlers := []app.HandlerFunc{Recovery(), handler}
	c.SetHandlers(handlers)
	handlers[0](context.Background(), c)
	return c
}

// 回归 #5：panic 值与栈必须落 ERROR 日志，不能被静默吞掉。
func TestRecoveryLogsPanicWithStack(t *testing.T) {
	sb := setupLogger(t)

	c := runRecovery(t, "/boom", func(ctx context.Context, c *app.RequestContext) {
		panic("boom in handler")
	})

	if got := c.Response.StatusCode(); got != 500 {
		t.Errorf("panic 兜底后 HTTP = %d, want 500", got)
	}
	body := string(c.Response.Body())
	if !strings.Contains(body, "Internal server error") {
		t.Errorf("响应应保持固定文案, got: %s", body)
	}
	if strings.Contains(body, "boom in handler") {
		t.Error("panic 值不应回给客户端，避免泄漏内部信息")
	}

	logs := sb.String()
	for _, want := range []string{"panic recovered", "boom in handler", "goroutine"} {
		if !strings.Contains(logs, want) {
			t.Errorf("日志缺少 %q, got: %s", want, logs)
		}
	}
}

// 回归 #5：日志里的 path 不带 query（query 可能含 token）。
func TestRecoveryLogPathExcludesQuery(t *testing.T) {
	sb := setupLogger(t)

	runRecovery(t, "/boom?token=topsecret", func(ctx context.Context, c *app.RequestContext) {
		panic("boom")
	})

	logs := sb.String()
	if !strings.Contains(logs, "/boom") {
		t.Errorf("日志应包含 path /boom, got: %s", logs)
	}
	if strings.Contains(logs, "token=topsecret") {
		t.Error("日志不应包含 query（可能含凭据）")
	}
}

// logger 未初始化时 Recovery 也不能二次 panic 或静默。
func TestRecoveryWithNilLogger(t *testing.T) {
	log.Logger = nil
	t.Cleanup(func() { log.Logger = nil })

	c := runRecovery(t, "/boom", func(ctx context.Context, c *app.RequestContext) {
		panic("boom")
	})

	if got := c.Response.StatusCode(); got != 500 {
		t.Errorf("panic 兜底后 HTTP = %d, want 500", got)
	}
}
