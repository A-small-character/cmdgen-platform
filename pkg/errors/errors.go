package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func (e *AppError) HTTPStatus() int {
	switch {
	case e.Code >= 1000 && e.Code < 2000:
		return http.StatusBadRequest
	case e.Code >= 2000 && e.Code < 3000:
		return http.StatusUnauthorized
	case e.Code >= 3000 && e.Code < 4000:
		return http.StatusForbidden
	case e.Code >= 4000 && e.Code < 5000:
		return http.StatusNotFound
	case e.Code >= 5000 && e.Code < 6000:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

var (
	ErrInvalidRequest    = &AppError{Code: 1000, Message: "无效请求"}
	ErrValidation        = &AppError{Code: 1001, Message: "参数验证失败"}
	ErrUnauthorized      = &AppError{Code: 2000, Message: "未授权"}
	ErrTokenExpired      = &AppError{Code: 2001, Message: "Token已过期"}
	ErrForbidden         = &AppError{Code: 3000, Message: "权限不足"}
	ErrNotFound          = &AppError{Code: 4000, Message: "资源不存在"}
	ErrInternal          = &AppError{Code: 5000, Message: "服务器内部错误"}
	ErrAIProviderFailed  = &AppError{Code: 5001, Message: "AI服务调用失败"}
	ErrVectorSearch      = &AppError{Code: 5002, Message: "向量搜索失败"}
	ErrDocCrawl          = &AppError{Code: 5003, Message: "文档抓取失败"}
	ErrPluginLoad        = &AppError{Code: 5004, Message: "插件加载失败"}
	ErrCommandGenerate   = &AppError{Code: 5005, Message: "命令生成失败"}
)

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(err error, code int, message string) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func WithDetail(base *AppError, detail string) *AppError {
	return &AppError{Code: base.Code, Message: base.Message, Detail: detail, Err: base.Err}
}
