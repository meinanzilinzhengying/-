//go:build linux

package errors

import (
	"encoding/json"
	"net/http"
	"time"
)

// Response 统一响应结构
type Response struct {
	Code      ErrorCode   `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	TraceID   string      `json:"trace_id"`
	Timestamp int64       `json:"timestamp"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Detail    string    `json:"detail,omitempty"`
	TraceID   string    `json:"trace_id"`
	Timestamp int64     `json:"timestamp"`
}

// Success 创建成功响应
func Success(data interface{}, traceID string) *Response {
	return &Response{
		Code:      CodeSuccess,
		Message:   "success",
		Data:      data,
		TraceID:   traceID,
		Timestamp: time.Now().Unix(),
	}
}

// SuccessWithMessage 创建带消息的成功响应
func SuccessWithMessage(data interface{}, message string, traceID string) *Response {
	return &Response{
		Code:      CodeSuccess,
		Message:   message,
		Data:      data,
		TraceID:   traceID,
		Timestamp: time.Now().Unix(),
	}
}

// Fail 创建失败响应
func Fail(code ErrorCode, traceID string) *ErrorResponse {
	info, ok := errorCodeMap[code]
	if !ok {
		info = &ErrorInfo{
			Code:    code,
			Message: "unknown error",
		}
	}

	return &ErrorResponse{
		Code:      code,
		Message:   info.Message,
		TraceID:   traceID,
		Timestamp: time.Now().Unix(),
	}
}

// FailWithMessage 创建带自定义消息的失败响应
func FailWithMessage(code ErrorCode, message string, traceID string) *ErrorResponse {
	return &ErrorResponse{
		Code:      code,
		Message:   message,
		TraceID:   traceID,
		Timestamp: time.Now().Unix(),
	}
}

// FromError 从错误创建响应
func FromError(err error, traceID string) *ErrorResponse {
	if err == nil {
		return nil
	}

	if appErr, ok := err.(*AppError); ok {
		return &ErrorResponse{
			Code:      appErr.Code,
			Message:   appErr.Message,
			Detail:    appErr.Detail,
			TraceID:   traceID,
			Timestamp: time.Now().Unix(),
		}
	}

	return &ErrorResponse{
		Code:      CodeInternalError,
		Message:   "internal server error",
		Detail:    err.Error(),
		TraceID:   traceID,
		Timestamp: time.Now().Unix(),
	}
}

// WriteJSON 写入 JSON 响应
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteSuccess 写入成功响应
func WriteSuccess(w http.ResponseWriter, data interface{}, traceID string) {
	WriteJSON(w, http.StatusOK, Success(data, traceID))
}

// WriteError 写入错误响应
func WriteError(w http.ResponseWriter, err error, traceID string) {
	status := GetHTTPStatusFromError(err)
	resp := FromError(err, traceID)
	WriteJSON(w, status, resp)
}

// WriteErrorResponse 写入错误码响应
func WriteErrorResponse(w http.ResponseWriter, code ErrorCode, traceID string) {
	status := GetHTTPStatus(code)
	resp := Fail(code, traceID)
	WriteJSON(w, status, resp)
}

// WriteErrorWithMessage 写入带消息的错误响应
func WriteErrorWithMessage(w http.ResponseWriter, code ErrorCode, message string, traceID string) {
	status := GetHTTPStatus(code)
	resp := FailWithMessage(code, message, traceID)
	WriteJSON(w, status, resp)
}

// WriteBadRequest 写入参数错误响应
func WriteBadRequest(w http.ResponseWriter, detail string, traceID string) {
	resp := Fail(CodeInvalidParam, traceID)
	resp.Detail = detail
	WriteJSON(w, http.StatusBadRequest, resp)
}

// WriteNotFound 写入未找到响应
func WriteNotFound(w http.ResponseWriter, resource string, traceID string) {
	resp := FailWithMessage(CodeNotFound, resource+" not found", traceID)
	WriteJSON(w, http.StatusNotFound, resp)
}

// WriteUnauthorized 写入未授权响应
func WriteUnauthorized(w http.ResponseWriter, traceID string) {
	WriteErrorResponse(w, CodeUnauthorized, traceID)
}

// WriteForbidden 写入禁止访问响应
func WriteForbidden(w http.ResponseWriter, traceID string) {
	WriteErrorResponse(w, CodeForbidden, traceID)
}

// WriteInternalError 写入内部错误响应
func WriteInternalError(w http.ResponseWriter, detail string, traceID string) {
	resp := Fail(CodeInternalError, traceID)
	resp.Detail = detail
	WriteJSON(w, http.StatusInternalServerError, resp)
}

// JSON 快捷方法：返回 JSON 字符串
func (r *Response) JSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}

// JSON 快捷方法：返回 JSON 字符串
func (r *ErrorResponse) JSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}
