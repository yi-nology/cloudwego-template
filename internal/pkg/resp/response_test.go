package resp

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"{{.module_name}}/internal/pkg/errors"
)

// 回归 #4：业务错误码必须映射到正确的 HTTP 状态码
// （旧实现只映射了 Unauthorized，其余全部 HTTP 200）。
func TestErrorCodeToHTTPStatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		code       int
		wantStatus int
	}{
		{"BadRequest", errors.CodeBadRequest, 400},
		{"Unauthorized", errors.CodeUnauthorized, 401},
		{"Forbidden", errors.CodeForbidden, 403},
		{"NotFound", errors.CodeNotFound, 404},
		{"Conflict", errors.CodeConflict, 409},
		{"InternalError", errors.CodeInternalError, 500},
		{"ServiceUnavailable", errors.CodeServiceUnavailable, 503},
		{"TooManyRequests", errors.CodeTooManyRequests, 429},
		{"未知错误码 fail-loud", 9999, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, fn := range []struct {
				name string
				call func(c *app.RequestContext)
			}{
				{"Error", func(c *app.RequestContext) { Error(c, tc.code) }},
				{"ErrorWithMessage", func(c *app.RequestContext) { ErrorWithMessage(c, tc.code, "msg") }},
				{"ErrorWithData", func(c *app.RequestContext) { ErrorWithData(c, tc.code, "msg", nil) }},
			} {
				c := app.NewContext(8)
				fn.call(c)
				if got := c.Response.StatusCode(); got != tc.wantStatus {
					t.Errorf("%s(code=%d) HTTP = %d, want %d", fn.name, tc.code, got, tc.wantStatus)
				}
			}
		})
	}
}

// Recovery 等兜底路径使用的 InternalError 也必须是 HTTP 500。
func TestInternalErrorHelperMapsTo500(t *testing.T) {
	c := app.NewContext(8)
	InternalError(c, "Internal server error")
	if got := c.Response.StatusCode(); got != 500 {
		t.Errorf("InternalError HTTP = %d, want 500", got)
	}
}
