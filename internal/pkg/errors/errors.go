package errors

import stderrors "errors"

const (
	CodeSuccess            = 0
	CodeBadRequest         = 1001
	CodeUnauthorized       = 1002
	CodeForbidden          = 1003
	CodeNotFound           = 1004
	CodeConflict           = 1005
	CodeInternalError      = 1006
	CodeServiceUnavailable = 1007
	CodeTooManyRequests    = 1008
)

var codeMessages = map[int]string{
	CodeSuccess:            "success",
	CodeBadRequest:         "bad request",
	CodeUnauthorized:       "unauthorized",
	CodeForbidden:          "forbidden",
	CodeNotFound:           "not found",
	CodeConflict:           "conflict",
	CodeInternalError:      "internal server error",
	CodeServiceUnavailable: "service unavailable",
	CodeTooManyRequests:    "too many requests",
}

type AppError struct {
	Code    int
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code int, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func GetMessage(code int) string {
	if msg, ok := codeMessages[code]; ok {
		return msg
	}
	return "unknown error"
}

func BadRequest(message string) *AppError {
	if message == "" {
		message = GetMessage(CodeBadRequest)
	}
	return New(CodeBadRequest, message)
}

func Unauthorized(message string) *AppError {
	if message == "" {
		message = GetMessage(CodeUnauthorized)
	}
	return New(CodeUnauthorized, message)
}

func Forbidden(message string) *AppError {
	if message == "" {
		message = GetMessage(CodeForbidden)
	}
	return New(CodeForbidden, message)
}

func NotFound(message string) *AppError {
	if message == "" {
		message = GetMessage(CodeNotFound)
	}
	return New(CodeNotFound, message)
}

func Conflict(message string) *AppError {
	if message == "" {
		message = GetMessage(CodeConflict)
	}
	return New(CodeConflict, message)
}

func InternalError(message string) *AppError {
	if message == "" {
		message = GetMessage(CodeInternalError)
	}
	return New(CodeInternalError, message)
}

func IsAppError(err error) bool {
	var appErr *AppError
	return stderrors.As(err, &appErr)
}

func GetErrorCode(err error) int {
	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternalError
}

// HTTPStatusOf 把业务错误码映射为 HTTP 状态码。
// 未收录的非成功码按内部错误处理（fail-loud），
// 避免业务错误全部以 HTTP 200 逃过监控、网关与客户端的重试/分流逻辑。
func HTTPStatusOf(code int) int {
	switch code {
	case CodeSuccess:
		return 200
	case CodeBadRequest:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeInternalError:
		return 500
	case CodeServiceUnavailable:
		return 503
	case CodeTooManyRequests:
		return 429
	default:
		return 500
	}
}
