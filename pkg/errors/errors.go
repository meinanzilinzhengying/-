// Package errors 提供统一错误码体系
// 支持前后端/服务间错误信息统一，便于排查问题
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// ErrorCode 错误码类型
type ErrorCode int

// ==================== 错误码定义 ====================
// 错误码格式: XYYZZ
// X: 服务类型 (1=通用, 2=Agent, 3=Server, 4=Edge, 5=Storage)
// YY: 模块类型
// ZZ: 具体错误

const (
	// 成功
	OK ErrorCode = 0

	// ========== 通用错误 (100000 - 100999) ==========
	// 基础错误 (100000 - 100099)
	ErrUnknown           ErrorCode = 100000 // 未知错误
	ErrInternal          ErrorCode = 100001 // 内部错误
	ErrInvalidArgument   ErrorCode = 100002 // 参数无效
	ErrMissingArgument   ErrorCode = 100003 // 缺少参数
	ErrInvalidFormat     ErrorCode = 100004 // 格式错误
	ErrDeadlineExceeded  ErrorCode = 100005 // 超时
	ErrCanceled          ErrorCode = 100006 // 已取消
	ErrNotFound          ErrorCode = 100007 // 未找到
	ErrAlreadyExists     ErrorCode = 100008 // 已存在
	ErrPermissionDenied  ErrorCode = 100009 // 权限不足
	ErrResourceExhausted ErrorCode = 100010 // 资源耗尽
	ErrUnavailable       ErrorCode = 100011 // 服务不可用
	ErrDataLoss          ErrorCode = 100012 // 数据丢失
	ErrUnauthenticated   ErrorCode = 100013 // 未认证
	ErrUnimplemented     ErrorCode = 100014 // 未实现
	ErrOutOfRange        ErrorCode = 100015 // 超出范围
	ErrAborted           ErrorCode = 100016 // 已中止

	// 网络错误 (100100 - 100199)
	ErrNetworkConnect    ErrorCode = 100100 // 连接失败
	ErrNetworkTimeout    ErrorCode = 100101 // 网络超时
	ErrNetworkDNS        ErrorCode = 100102 // DNS错误
	ErrNetworkTLS        ErrorCode = 100103 // TLS错误
	ErrNetworkCircuit    ErrorCode = 100104 // 熔断器开启
	ErrNetworkRateLimit  ErrorCode = 100105 // 限流

	// 数据错误 (100200 - 100299)
	ErrDataEncode        ErrorCode = 100200 // 编码错误
	ErrDataDecode        ErrorCode = 100201 // 解码错误
	ErrDataCompress      ErrorCode = 100202 // 压缩错误
	ErrDataEncrypt       ErrorCode = 100203 // 加密错误
	ErrDataValidation    ErrorCode = 100204 // 数据验证失败
	ErrDataConflict      ErrorCode = 100205 // 数据冲突

	// ========== Agent 错误 (200000 - 299999) ==========
	// Agent 基础错误 (200000 - 200099)
	ErrAgentNotRegistered     ErrorCode = 200000 // Agent 未注册
	ErrAgentAlreadyRegistered ErrorCode = 200001 // Agent 已注册
	ErrAgentNotRunning        ErrorCode = 200002 // Agent 未运行
	ErrAgentAlreadyRunning    ErrorCode = 200003 // Agent 已在运行
	ErrAgentVersionMismatch   ErrorCode = 200004 // 版本不匹配
	ErrAgentHeartbeatTimeout  ErrorCode = 200005 // 心跳超时
	ErrAgentConfigInvalid     ErrorCode = 200006 // 配置无效
	ErrAgentConfigApplyFailed ErrorCode = 200007 // 配置应用失败
	ErrAgentUpgradeFailed     ErrorCode = 200008 // 升级失败
	ErrAgentRestartFailed     ErrorCode = 200009 // 重启失败

	// Agent eBPF 错误 (200100 - 200199)
	ErrAgentEBPFLoadFailed   ErrorCode = 200100 // eBPF 加载失败
	ErrAgentEBPFAttachFailed ErrorCode = 200101 // eBPF 附加失败
	ErrAgentEBPFDetachFailed ErrorCode = 200102 // eBPF 分离失败
	ErrAgentKernelNotSupport ErrorCode = 200103 // 内核不支持
	ErrAgentPrivilegeInsufficient ErrorCode = 200104 // 权限不足

	// Agent 采集器错误 (200200 - 200299)
	ErrAgentCollectorStartFailed ErrorCode = 200200 // 采集器启动失败
	ErrAgentCollectorStopFailed  ErrorCode = 200201 // 采集器停止失败
	ErrAgentCollectorNotFound    ErrorCode = 200202 // 采集器未找到
	ErrAgentCollectorConfigInvalid ErrorCode = 200203 // 采集器配置无效
	ErrAgentCollectorResourceLimit ErrorCode = 200204 // 采集器资源限制

	// Agent 数据采集错误 (200300 - 200399)
	ErrAgentDataCollectFailed  ErrorCode = 200300 // 数据采集失败
	ErrAgentDataBufferFull     ErrorCode = 200301 // 数据缓冲区满
	ErrAgentDataDrop           ErrorCode = 200302 // 数据丢弃
	ErrAgentDataSendFailed     ErrorCode = 200303 // 数据发送失败
	ErrAgentDataRateLimited    ErrorCode = 200304 // 数据速率限制

	// ========== Server 错误 (300000 - 399999) ==========
	// Server 基础错误 (300000 - 300099)
	ErrServerInternal      ErrorCode = 300000 // 服务器内部错误
	ErrServerUnavailable   ErrorCode = 300001 // 服务器不可用
	ErrServerOverload      ErrorCode = 300002 // 服务器过载
	ErrServerMaintenance   ErrorCode = 300003 // 服务器维护中

	// Server 认证授权错误 (300100 - 300199)
	ErrServerAuthTokenInvalid    ErrorCode = 300100 // Token 无效
	ErrServerAuthTokenExpired    ErrorCode = 300101 // Token 过期
	ErrServerAuthSignatureInvalid ErrorCode = 300102 // 签名无效
	ErrServerAuthInsufficient    ErrorCode = 300103 // 权限不足
	ErrServerAuthAccountLocked   ErrorCode = 300104 // 账户锁定
	ErrServerAuthAccountDisabled ErrorCode = 300105 // 账户禁用
	ErrServerAuthMFARequired     ErrorCode = 300106 // 需要 MFA
	ErrServerAuthMFAInvalid      ErrorCode = 300107 // MFA 无效

	// Server 配置错误 (300200 - 300299)
	ErrServerConfigParseFailed    ErrorCode = 300200 // 配置解析失败
	ErrServerConfigInvalid        ErrorCode = 300201 // 配置无效
	ErrServerConfigVersionMismatch ErrorCode = 300202 // 配置版本不匹配
	ErrServerConfigNotFound       ErrorCode = 300203 // 配置未找到

	// Server 租户错误 (300300 - 300399)
	ErrServerTenantNotFound      ErrorCode = 300300 // 租户未找到
	ErrServerTenantQuotaExceeded ErrorCode = 300301 // 租户配额超限
	ErrServerTenantExpired       ErrorCode = 300302 // 租户已过期
	ErrServerTenantDisabled      ErrorCode = 300303 // 租户已禁用

	// Server 用户错误 (300400 - 300499)
	ErrServerUserNotFound      ErrorCode = 300400 // 用户未找到
	ErrServerUserAlreadyExists ErrorCode = 300401 // 用户已存在
	ErrServerUserPasswordInvalid ErrorCode = 300402 // 密码无效

	// Server 策略错误 (300500 - 300599)
	ErrServerPolicyNotFound   ErrorCode = 300500 // 策略未找到
	ErrServerPolicyInvalid    ErrorCode = 300501 // 策略无效
	ErrServerPolicyConflict   ErrorCode = 300502 // 策略冲突

	// Server 告警错误 (300600 - 300699)
	ErrServerAlertRuleInvalid      ErrorCode = 300600 // 告警规则无效
	ErrServerAlertNotificationFailed ErrorCode = 300601 // 告警通知失败
	ErrServerAlertSilenced         ErrorCode = 300602 // 告警已静默

	// ========== Edge 错误 (400000 - 499999) ==========
	// Edge 基础错误 (400000 - 400099)
	ErrEdgeInternal      ErrorCode = 400000 // Edge 内部错误
	ErrEdgeUnavailable   ErrorCode = 400001 // Edge 不可用
	ErrEdgeOverload      ErrorCode = 400002 // Edge 过载

	// Edge 路由错误 (400100 - 400199)
	ErrEdgeRoutingFailed    ErrorCode = 400100 // 路由失败
	ErrEdgeNoAvailableNode  ErrorCode = 400101 // 无可用节点
	ErrEdgeNodeNotFound     ErrorCode = 400102 // 节点未找到

	// Edge 服务发现错误 (400200 - 400299)
	ErrEdgeServiceNotFound      ErrorCode = 400200 // 服务未找到
	ErrEdgeServiceAlreadyExists ErrorCode = 400201 // 服务已存在
	ErrEdgeServiceExpired       ErrorCode = 400202 // 服务已过期

	// ========== Storage 错误 (500000 - 599999) ==========
	// Storage 基础错误 (500000 - 500099)
	ErrStorageInternal       ErrorCode = 500000 // 存储内部错误
	ErrStorageUnavailable    ErrorCode = 500001 // 存储不可用
	ErrStorageConnectionFailed ErrorCode = 500002 // 存储连接失败

	// Storage 读写错误 (500100 - 500199)
	ErrStorageWriteFailed  ErrorCode = 500100 // 写入失败
	ErrStorageReadFailed   ErrorCode = 500101 // 读取失败
	ErrStorageDeleteFailed ErrorCode = 500102 // 删除失败

	// Storage 配额错误 (500200 - 500299)
	ErrStorageQuotaExceeded ErrorCode = 500200 // 存储配额超限
	ErrStorageRateLimited   ErrorCode = 500201 // 存储限流
)

// ==================== 错误信息映射 ====================

var errorMessages = map[ErrorCode]string{
	OK: "成功",

	// 通用错误
	ErrUnknown:           "未知错误",
	ErrInternal:          "内部错误",
	ErrInvalidArgument:   "参数无效",
	ErrMissingArgument:   "缺少参数",
	ErrInvalidFormat:     "格式错误",
	ErrDeadlineExceeded:  "请求超时",
	ErrCanceled:          "请求已取消",
	ErrNotFound:          "资源未找到",
	ErrAlreadyExists:     "资源已存在",
	ErrPermissionDenied:  "权限不足",
	ErrResourceExhausted: "资源耗尽",
	ErrUnavailable:       "服务不可用",
	ErrDataLoss:          "数据丢失",
	ErrUnauthenticated:   "未认证",
	ErrUnimplemented:     "功能未实现",
	ErrOutOfRange:        "超出范围",
	ErrAborted:           "操作已中止",

	// 网络错误
	ErrNetworkConnect:   "网络连接失败",
	ErrNetworkTimeout:   "网络超时",
	ErrNetworkDNS:       "DNS解析失败",
	ErrNetworkTLS:       "TLS握手失败",
	ErrNetworkCircuit:   "熔断器已开启",
	ErrNetworkRateLimit: "请求被限流",

	// 数据错误
	ErrDataEncode:     "数据编码失败",
	ErrDataDecode:     "数据解码失败",
	ErrDataCompress:   "数据压缩失败",
	ErrDataEncrypt:    "数据加密失败",
	ErrDataValidation: "数据验证失败",
	ErrDataConflict:   "数据冲突",

	// Agent 错误
	ErrAgentNotRegistered:     "Agent未注册",
	ErrAgentAlreadyRegistered: "Agent已注册",
	ErrAgentNotRunning:        "Agent未运行",
	ErrAgentAlreadyRunning:    "Agent已在运行",
	ErrAgentVersionMismatch:   "Agent版本不匹配",
	ErrAgentHeartbeatTimeout:  "Agent心跳超时",
	ErrAgentConfigInvalid:     "Agent配置无效",
	ErrAgentConfigApplyFailed: "Agent配置应用失败",
	ErrAgentUpgradeFailed:     "Agent升级失败",
	ErrAgentRestartFailed:     "Agent重启失败",

	// Agent eBPF 错误
	ErrAgentEBPFLoadFailed:        "eBPF加载失败",
	ErrAgentEBPFAttachFailed:      "eBPF附加失败",
	ErrAgentEBPFDetachFailed:      "eBPF分离失败",
	ErrAgentKernelNotSupport:      "内核不支持",
	ErrAgentPrivilegeInsufficient: "权限不足",

	// Agent 采集器错误
	ErrAgentCollectorStartFailed:   "采集器启动失败",
	ErrAgentCollectorStopFailed:    "采集器停止失败",
	ErrAgentCollectorNotFound:      "采集器未找到",
	ErrAgentCollectorConfigInvalid: "采集器配置无效",
	ErrAgentCollectorResourceLimit: "采集器资源限制",

	// Agent 数据采集错误
	ErrAgentDataCollectFailed: "数据采集失败",
	ErrAgentDataBufferFull:    "数据缓冲区满",
	ErrAgentDataDrop:          "数据丢弃",
	ErrAgentDataSendFailed:    "数据发送失败",
	ErrAgentDataRateLimited:   "数据速率限制",

	// Server 错误
	ErrServerInternal:      "服务器内部错误",
	ErrServerUnavailable:   "服务器不可用",
	ErrServerOverload:      "服务器过载",
	ErrServerMaintenance:   "服务器维护中",

	// Server 认证授权错误
	ErrServerAuthTokenInvalid:     "Token无效",
	ErrServerAuthTokenExpired:     "Token已过期",
	ErrServerAuthSignatureInvalid: "签名无效",
	ErrServerAuthInsufficient:     "权限不足",
	ErrServerAuthAccountLocked:    "账户已锁定",
	ErrServerAuthAccountDisabled:  "账户已禁用",
	ErrServerAuthMFARequired:      "需要多因素认证",
	ErrServerAuthMFAInvalid:       "多因素认证无效",

	// Server 配置错误
	ErrServerConfigParseFailed:     "配置解析失败",
	ErrServerConfigInvalid:         "配置无效",
	ErrServerConfigVersionMismatch: "配置版本不匹配",
	ErrServerConfigNotFound:        "配置未找到",

	// Server 租户错误
	ErrServerTenantNotFound:      "租户未找到",
	ErrServerTenantQuotaExceeded: "租户配额超限",
	ErrServerTenantExpired:       "租户已过期",
	ErrServerTenantDisabled:      "租户已禁用",

	// Server 用户错误
	ErrServerUserNotFound:        "用户未找到",
	ErrServerUserAlreadyExists:   "用户已存在",
	ErrServerUserPasswordInvalid: "密码无效",

	// Server 策略错误
	ErrServerPolicyNotFound: "策略未找到",
	ErrServerPolicyInvalid:  "策略无效",
	ErrServerPolicyConflict: "策略冲突",

	// Server 告警错误
	ErrServerAlertRuleInvalid:       "告警规则无效",
	ErrServerAlertNotificationFailed: "告警通知失败",
	ErrServerAlertSilenced:          "告警已静默",

	// Edge 错误
	ErrEdgeInternal:      "Edge内部错误",
	ErrEdgeUnavailable:   "Edge不可用",
	ErrEdgeOverload:      "Edge过载",
	ErrEdgeRoutingFailed: "路由失败",
	ErrEdgeNoAvailableNode: "无可用节点",
	ErrEdgeNodeNotFound:  "节点未找到",

	// Edge 服务发现错误
	ErrEdgeServiceNotFound:      "服务未找到",
	ErrEdgeServiceAlreadyExists: "服务已存在",
	ErrEdgeServiceExpired:       "服务已过期",

	// Storage 错误
	ErrStorageInternal:         "存储内部错误",
	ErrStorageUnavailable:      "存储不可用",
	ErrStorageConnectionFailed: "存储连接失败",
	ErrStorageWriteFailed:      "写入失败",
	ErrStorageReadFailed:       "读取失败",
	ErrStorageDeleteFailed:     "删除失败",
	ErrStorageQuotaExceeded:    "存储配额超限",
	ErrStorageRateLimited:      "存储限流",
}

// ==================== 错误码到 HTTP 状态码映射 ====================

var errorCodeToHTTPStatus = map[ErrorCode]int{
	OK: http.StatusOK,

	ErrInvalidArgument:   http.StatusBadRequest,
	ErrMissingArgument:   http.StatusBadRequest,
	ErrInvalidFormat:     http.StatusBadRequest,
	ErrNotFound:          http.StatusNotFound,
	ErrAlreadyExists:     http.StatusConflict,
	ErrPermissionDenied:  http.StatusForbidden,
	ErrUnauthenticated:   http.StatusUnauthorized,
	ErrUnavailable:       http.StatusServiceUnavailable,
	ErrDeadlineExceeded:  http.StatusGatewayTimeout,
	ErrResourceExhausted: http.StatusTooManyRequests,
	ErrUnimplemented:     http.StatusNotImplemented,

	ErrNetworkConnect:   http.StatusBadGateway,
	ErrNetworkTimeout:   http.StatusGatewayTimeout,
	ErrNetworkRateLimit: http.StatusTooManyRequests,

	ErrServerAuthTokenInvalid:     http.StatusUnauthorized,
	ErrServerAuthTokenExpired:     http.StatusUnauthorized,
	ErrServerAuthSignatureInvalid: http.StatusUnauthorized,
	ErrServerAuthInsufficient:     http.StatusForbidden,
	ErrServerAuthAccountLocked:    http.StatusForbidden,
	ErrServerAuthAccountDisabled:  http.StatusForbidden,
	ErrServerAuthMFARequired:      http.StatusUnauthorized,
	ErrServerAuthMFAInvalid:       http.StatusUnauthorized,

	ErrServerTenantQuotaExceeded: http.StatusForbidden,
	ErrServerTenantExpired:       http.StatusForbidden,
	ErrServerTenantDisabled:      http.StatusForbidden,

	ErrStorageQuotaExceeded: http.StatusInsufficientStorage,
	ErrStorageRateLimited:   http.StatusTooManyRequests,
}

// ==================== Error 结构体 ====================

// Error 统一错误结构
type Error struct {
	Code       ErrorCode         `json:"code"`
	Message    string            `json:"message"`
	Detail     string            `json:"detail,omitempty"`
	RequestID  string            `json:"request_id,omitempty"`
	TraceID    string            `json:"trace_id,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Cause      error             `json:"-"`
	StackTrace []string          `json:"stack_trace,omitempty"`
}

// Error 实现 error 接口
func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 返回原始错误
func (e *Error) Unwrap() error {
	return e.Cause
}

// WithDetail 添加详细错误信息
func (e *Error) WithDetail(detail string) *Error {
	e.Detail = detail
	return e
}

// WithRequestID 添加请求ID
func (e *Error) WithRequestID(requestID string) *Error {
	e.RequestID = requestID
	return e
}

// WithTraceID 添加追踪ID
func (e *Error) WithTraceID(traceID string) *Error {
	e.TraceID = traceID
	return e
}

// WithField 添加字段
func (e *Error) WithField(key string, value interface{}) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	e.Fields[key] = value
	return e
}

// WithFields 添加多个字段
func (e *Error) WithFields(fields map[string]interface{}) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

// WithCause 添加原始错误
func (e *Error) WithCause(err error) *Error {
	e.Cause = err
	if err != nil {
		e.Detail = err.Error()
	}
	return e
}

// WithStack 添加堆栈信息
func (e *Error) WithStack() *Error {
	e.StackTrace = getStackTrace(2)
	return e
}

// Is 判断错误码是否匹配
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}

// HTTPStatus 返回对应的 HTTP 状态码
func (e *Error) HTTPStatus() int {
	if status, ok := errorCodeToHTTPStatus[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// ToJSON 转换为 JSON 字符串
func (e *Error) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// ToMap 转换为 map
func (e *Error) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"code":    e.Code,
		"message": e.Message,
		"detail":  e.Detail,
	}
}

// ==================== 错误创建函数 ====================

// New 创建新的错误
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewWithCode 根据错误码创建错误
func NewWithCode(code ErrorCode) *Error {
	message := GetMessage(code)
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装原始错误
func Wrap(err error, code ErrorCode, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:    code,
		Message: message,
		Detail:  err.Error(),
		Cause:   err,
	}
}

// WrapWithCode 使用错误码包装原始错误
func WrapWithCode(err error, code ErrorCode) *Error {
	if err == nil {
		return nil
	}
	message := GetMessage(code)
	return &Error{
		Code:    code,
		Message: message,
		Detail:  err.Error(),
		Cause:   err,
	}
}

// FromError 从 error 转换为 Error
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{
		Code:    ErrUnknown,
		Message: GetMessage(ErrUnknown),
		Detail:  err.Error(),
		Cause:   err,
	}
}

// ==================== 便捷创建函数 ====================

// BadRequest 创建参数错误
func BadRequest(detail string) *Error {
	return NewWithCode(ErrInvalidArgument).WithDetail(detail).WithStack()
}

// NotFound 创建未找到错误
func NotFound(resource string) *Error {
	return NewWithCode(ErrNotFound).WithDetail(resource).WithStack()
}

// AlreadyExists 创建已存在错误
func AlreadyExists(resource string) *Error {
	return NewWithCode(ErrAlreadyExists).WithDetail(resource).WithStack()
}

// PermissionDenied 创建权限错误
func PermissionDenied(detail string) *Error {
	return NewWithCode(ErrPermissionDenied).WithDetail(detail).WithStack()
}

// Unauthorized 创建未认证错误
func Unauthorized(detail string) *Error {
	return NewWithCode(ErrUnauthenticated).WithDetail(detail).WithStack()
}

// Internal 创建内部错误
func Internal(detail string) *Error {
	return NewWithCode(ErrInternal).WithDetail(detail).WithStack()
}

// Timeout 创建超时错误
func Timeout(detail string) *Error {
	return NewWithCode(ErrDeadlineExceeded).WithDetail(detail).WithStack()
}

// Unavailable 创建服务不可用错误
func Unavailable(detail string) *Error {
	return NewWithCode(ErrUnavailable).WithDetail(detail).WithStack()
}

// ==================== 工具函数 ====================

// GetMessage 获取错误码对应的消息
func GetMessage(code ErrorCode) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return "未知错误"
}

// GetHTTPStatus 获取错误码对应的 HTTP 状态码
func GetHTTPStatus(code ErrorCode) int {
	if status, ok := errorCodeToHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// IsError 判断错误是否为指定的错误码
func IsError(err error, code ErrorCode) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}

// IsNotFound 判断是否为未找到错误
func IsNotFound(err error) bool {
	return IsError(err, ErrNotFound)
}

// IsAlreadyExists 判断是否为已存在错误
func IsAlreadyExists(err error) bool {
	return IsError(err, ErrAlreadyExists)
}

// IsPermissionDenied 判断是否为权限错误
func IsPermissionDenied(err error) bool {
	return IsError(err, ErrPermissionDenied)
}

// IsTimeout 判断是否为超时错误
func IsTimeout(err error) bool {
	return IsError(err, ErrDeadlineExceeded)
}

// IsUnavailable 判断是否为服务不可用错误
func IsUnavailable(err error) bool {
	return IsError(err, ErrUnavailable)
}

// getStackTrace 获取堆栈信息
func getStackTrace(skip int) []string {
	var stack []string
	for i := skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			break
		}
		stack = append(stack, fmt.Sprintf("%s:%d %s", file, line, fn.Name()))
		if len(stack) >= 20 {
			break
		}
	}
	return stack
}

// ==================== 标准库 error 包装 ====================

// NewStandard 创建标准库 error
func NewStandard(message string) error {
	return errors.New(message)
}

// Is 判断错误是否匹配
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As 类型断言
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Unwrap 解包错误
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// Join 合并多个错误
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// ==================== 响应辅助函数 ====================

// Response 错误响应结构
type Response struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Detail  string                 `json:"detail,omitempty"`
	Data    interface{}            `json:"data,omitempty"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// NewResponse 创建错误响应
func NewResponse(err *Error) *Response {
	if err == nil {
		return &Response{
			Code:    OK,
			Message: GetMessage(OK),
		}
	}
	return &Response{
		Code:    err.Code,
		Message: err.Message,
		Detail:  err.Detail,
		Fields:  err.Fields,
	}
}

// SuccessResponse 创建成功响应
func SuccessResponse(data interface{}) *Response {
	return &Response{
		Code:    OK,
		Message: GetMessage(OK),
		Data:    data,
	}
}

// JSON 转换为 JSON 字节
func (r *Response) JSON() []byte {
	data, _ := json.Marshal(r)
	return data
}

// JSONString 转换为 JSON 字符串
func (r *Response) JSONString() string {
	return string(r.JSON())
}
