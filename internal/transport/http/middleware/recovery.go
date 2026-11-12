package middleware

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/cloudwego/hertz/pkg/app"
	zap "go.uber.org/zap"

	log "{{.module_name}}/internal/pkg/logger"
	resp "{{.module_name}}/internal/pkg/resp"
)

// Recovery 兜住 handler panic：
//   - panic 值与栈必须落 ERROR 日志（旧实现完全吞掉，线上无法定位）；
//   - 响应保持固定文案，不把 panic 值回给客户端，避免泄漏内部信息；
//   - path 只取 URI path，不带 query（query 可能含 token 等凭据）。
func Recovery() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				logPanic(c, err)
				resp.InternalError(c, "Internal server error")
				c.Abort()
			}
		}()
		c.Next(ctx)
	}
}

func logPanic(c *app.RequestContext, err any) {
	fields := []zap.Field{
		zap.String("method", string(c.Method())),
		zap.String("path", string(c.URI().Path())),
		zap.Any("panic", err),
		zap.ByteString("stack", debug.Stack()),
	}
	// logger 未初始化时也不吞 panic：退回 stderr，保持 fail-loud
	if l := log.Logger; l != nil {
		l.Error("panic recovered", fields...)
		return
	}
	fmt.Fprintf(os.Stderr, "panic recovered: method=%s path=%s panic=%v\n%s",
		string(c.Method()), string(c.URI().Path()), err, debug.Stack())
}
