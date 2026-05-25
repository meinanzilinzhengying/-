//go:build linux

// Package errors 提供统一错误码和错误处理
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ============================================================
// 错误码定义
// ============================================================

// ErrorCode 错误码类型
type ErrorCode int

const (
	// 成功
	CodeSuccess ErrorCode = 0

	// 通用错误 (1-999)
	CodeInternalError     ErrorCode = 1   // 内部错误
	CodeInvalidParam      ErrorCode = 2   // 参数错误
	CodeUnauthorized      ErrorCode = 3   // 未授权
	CodeForbidden         ErrorCode = 4   // 禁止访问
	CodeNotFound          ErrorCode = 5   // 资源不存在
	CodeAlreadyExists     ErrorCode = 6   // 资源已存在
	CodeTooManyRequests   ErrorCode = 7   // 请求过于频繁
	CodeServiceUnavailable ErrorCode = 8  // 服务不可用
	CodeTimeout           ErrorCode = 9   // 请求超时

	// 业务错误 (1000-1999)
	CodeInvalidConfig     ErrorCode = 1000 // 配置错误
	CodeDatabaseError     ErrorCode = 1001 // 数据库错误
	CodeCacheError        ErrorCode = 1002 // 缓存错误
	CodeNetworkError      ErrorCode = 1003 // 网络错误
	CodeThirdPartyError   ErrorCode = 1004 // 第三方服务错误

	// Agent 相关错误 (2000-2999)
	CodeAgentNotFound     ErrorCode = 2000 // Agent 不存在
	CodeAgentOffline      ErrorCode = 2001 // Agent 离线
	CodeAgentRegisterFail ErrorCode = 2002 // Agent 注册失败

	// 数据相关错误 (3000-3999)
	CodeDataNotFound      ErrorCode = 3000 // 数据不存在
	CodeDataInvalid       ErrorCode = 3001 // 数据无效
	CodeDataExpired       ErrorCode = 3002 // 数据已过期
)

// 错误码映射表
var errorCodeMap = map[ErrorCode]*ErrorInfo{
	CodeSuccess:            {Code: CodeSuccess, Message: "success", HTTPStatus: http.StatusOK},
	CodeInternalError:      {Code: CodeInternalError, Message: "internal server error", HTTPStatus: http.StatusInternalServerError},
	CodeInvalidParam:       {Code: CodeInvalidParam, Message: "invalid parameter", HTTPStatus: http.StatusBadRequest},
	CodeUnauthorized:       {Code: CodeUnauthorized, Message: "unauthorized", HTTPStatus: http.StatusUnauthorized},
	CodeForbidden:          {Code: CodeForbidden, Message: "forbidden", HTTPStatus: http.StatusForbidden},
	CodeNotFound:           {Code: CodeNotFound, Message: "resource not found", HTTPStatus: http.StatusNotFound},
	CodeAlreadyExists:      {Code: CodeAlreadyExists, Message: "resource already exists", HTTPStatus: http.StatusConflict},
	CodeTooManyRequests:    {Code: CodeTooManyRequests, Message: "too many requests", HTTPStatus: http.StatusTooManyRequests},
	CodeServiceUnavailable: {Code: CodeServiceUnavailable, Message: "service unavailable", HTTPStatus: http.StatusServiceUnavailable},
	CodeTimeout:            {Code: CodeTimeout, Message: "request timeout", HTTPStatus: http.StatusGatewayTimeout},

	CodeInvalidConfig:     {Code: CodeInvalidConfig, Message: "invalid configuration", HTTPStatus: http.StatusInternalServerError},
	CodeDatabaseError:     {Code: CodeDatabaseError, Message: "database error", HTTPStatus: http.StatusInternalServerError},
	CodeCacheError:        {Code: CodeCacheError, Message: "cache error", HTTPStatus: http.StatusInternalServerError},
	CodeNetworkError:      {Code: CodeNetworkError, Message: "network error", HTTPStatus: http.StatusInternalServerError},
	CodeThirdPartyError:   {Code: CodeThirdPartyError, Message: "third party service error", HTTPStatus: http.StatusInternalServerError},

	CodeAgentNotFound:     {Code: CodeAgentNotFound, Message: "agent not found", HTTPStatus: http.StatusNotFound},
	CodeAgentOffline:      {Code: CodeAgentOffline, Message: "agent is offline", HTTPStatus: http.StatusServiceUnavailable},
	CodeAgentRegisterFail: {Code: CodeAgentRegisterFail, Message: "agent register failed", HTTPStatus: http.StatusInternalServerError},

	CodeDataNotFound:      {Code: CodeDataNotFound, Message: "data not found", HTTPStatus: http.StatusNotFound},
	CodeDataInvalid:       {Code: CodeDataInvalid, Message: "data invalid", HTTPStatus: http.StatusBadRequest},
	CodeDataExpired:       {Code: CodeDataExpired, message: "data expired", HTTPStatus: http.StatusBadRequest},
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	HTTPStatus int       `json:"-"`
}

// AppError 应用错误
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
	TraceID string    `json:"trace_id,omitempty"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	return fmt.Sprintf("[Code:%d] %s: %s", e.Code, e.Message, e.Detail)
}

// WithDetail 添加详细错误信息
func (e *AppError) WithDetail(detail string) *AppError {
	e.Detail = detail
	return e
}

// WithTraceID 添加 TraceID
func (e *AppError) WithTraceID(traceID string) *AppError {
	e.TraceID = traceID
	return e
}

// ============================================================
// 预定义错误
// ============================================================

var (
	// 通用错误
	ErrInternalServer    = NewError(CodeInternalError)
	ErrInvalidParam      = NewError(CodeInvalidParam)
	ErrUnauthorized      = NewError(CodeUnauthorized)
	ErrForbidden         = NewError(CodeForbidden)
	ErrNotFound          = NewError(CodeNotFound)
	ErrAlreadyExists     = NewError(CodeAlreadyExists)
	ErrTooManyRequests   = NewError(CodeTooManyRequests)
	ErrServiceUnavailable = NewError(CodeServiceUnavailable)
	ErrTimeout           = NewError(CodeTimeout)

	// 业务错误
	ErrInvalidConfig   = NewError(CodeInvalidConfig)
	ErrDatabase        = NewError(CodeDatabaseError)
	ErrCache           = NewError(CodeCacheError)
	ErrNetwork         = NewError(CodeNetworkError)
	ErrThirdParty      = NewError(CodeThirdPartyError)

	// Agent 错误
	ErrAgentNotFound     = NewError(CodeAgentNotFound)
	ErrAgentOffline      = NewError(CodeAgentOffline)
	ErrAgentRegisterFail = NewError(CodeAgentRegisterFail)

	// 数据错误
	ErrDataNotFound = NewError(CodeDataNotFound)
	ErrDataInvalid  = NewError(CodeDataInvalid)
	ErrDataExpired  = NewError(CodeDataExpired)
)

// NewError 创建错误
func NewError(code ErrorCode) *AppError {
	info, ok := errorCodeMap[code]
	if !ok {
		return &AppError{
			Code:    code,
			Message: "unknown error",
		}
	}
	return &AppError{
		Code:    code,
		Message: info.Message,
	}
}

// NewErrorWithMessage 创建带自定义消息的错误
func NewErrorWithMessage(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf 创建带格式化消息的错误
func NewErrorf(code ErrorCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap 包装错误
func Wrap(err error, code ErrorCode) *AppError {
	if err == nil {
		return nil
	}

	if appErr, ok := err.(*AppError); ok {
		return appErr
	}

	return NewError(code).WithDetail(err.Error())
}

// WrapWithMessage 包装错误并添加消息
func WrapWithMessage(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}

	return NewErrorWithMessage(code, message).WithDetail(err.Error())
}

// Is 判断错误码是否匹配
func Is(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}

	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}

	return false
}

// GetHTTPStatus 获取 HTTP 状态码
func GetHTTPStatus(code ErrorCode) int {
	if info, ok := errorCodeMap[code]; ok {
		return info.HTTPStatus
	}
	return http.StatusInternalServerError
}

// GetHTTPStatusFromError 从错误获取 HTTP 状态码
func GetHTTPStatusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	if appErr, ok := err.(*AppError); ok {
		return GetHTTPStatus(appErr.Code)
	}

	return http.StatusInternalServerError
}
