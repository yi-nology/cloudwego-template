package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

// 回归 #3：GetErrorCode / IsAppError 必须穿透包装链识别 AppError
// （旧实现用类型断言，包装一层后错误码静默塌成 1006）。
func TestWrappedAppErrorRecognized(t *testing.T) {
	appErr := NotFound("user not found")

	cases := []struct {
		name     string
		err      error
		wantCode int
		wantIs   bool
	}{
		{name: "未包装", err: appErr, wantCode: CodeNotFound, wantIs: true},
		{name: "包装一层", err: fmt.Errorf("查询用户 %d 失败: %w", 42, appErr), wantCode: CodeNotFound, wantIs: true},
		{
			name:     "包装两层",
			err:      fmt.Errorf("service: %w", fmt.Errorf("dao: %w", appErr)),
			wantCode: CodeNotFound,
			wantIs:   true,
		},
		{name: "外来错误", err: stderrors.New("boom"), wantCode: CodeInternalError, wantIs: false},
		{name: "nil", err: nil, wantCode: CodeInternalError, wantIs: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetErrorCode(tc.err); got != tc.wantCode {
				t.Errorf("GetErrorCode = %d, want %d", got, tc.wantCode)
			}
			if got := IsAppError(tc.err); got != tc.wantIs {
				t.Errorf("IsAppError = %v, want %v", got, tc.wantIs)
			}
		})
	}
}

// AppError 的 Unwrap 链应与标准库 errors.Is/As 协作。
func TestAppErrorUnwrapChain(t *testing.T) {
	cause := stderrors.New("root cause")
	wrapped := fmt.Errorf("ctx: %w", Wrap(CodeBadRequest, "bad", cause))

	if !stderrors.Is(wrapped, cause) {
		t.Error("errors.Is 应能沿 Unwrap 链找到根因")
	}
	var appErr *AppError
	if !stderrors.As(wrapped, &appErr) || appErr.Code != CodeBadRequest {
		t.Error("errors.As 应能识别包装链中的 AppError")
	}
}
