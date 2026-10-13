package helpers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Response codes.
const (
	CodeOK                  = "OK"
	CodeCreated             = "CREATED"
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeInternalServerError = "INTERNAL_SERVER_ERROR"
)

var statusCodes = map[int]string{
	http.StatusOK:                  CodeOK,
	http.StatusCreated:             CodeCreated,
	http.StatusBadRequest:          CodeBadRequest,
	http.StatusUnauthorized:        CodeUnauthorized,
	http.StatusForbidden:           CodeForbidden,
	http.StatusNotFound:            CodeNotFound,
	http.StatusUnprocessableEntity: CodeValidationError,
	http.StatusInternalServerError: CodeInternalServerError,
}

// CodeFromStatus returns the response code for an HTTP status code.
func CodeFromStatus(statusCode int) string {
	if code, ok := statusCodes[statusCode]; ok {
		return code
	}

	text := http.StatusText(statusCode)
	if text == "" {
		return "UNKNOWN"
	}

	return strings.ToUpper(strings.ReplaceAll(text, " ", "_"))
}

// PaginationMeta represents pagination metadata in response.
type PaginationMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// PaginatedResponseInterface defines the contract for paginated responses.
type PaginatedResponseInterface interface {
	GetData() interface{}
	GetMetadata() PaginationMeta
}

// Response represents a standard HTTP response structure
type Response struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Data    interface{}         `json:"data,omitempty"`
	Errors  map[string][]string `json:"errors,omitempty"`
}

// SuccessResponse sends a success response with optional data
func SuccessResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	SuccessResponseWithCode(c, statusCode, CodeFromStatus(statusCode), message, data)
}

// SuccessResponseWithCode sends a success response with a custom code.
func SuccessResponseWithCode(c *gin.Context, statusCode int, code string, message string, data interface{}) {
	resp := Response{
		Code:    code,
		Message: message,
		Data:    data,
	}
	c.JSON(statusCode, resp)
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, statusCode int, message string) {
	ErrorResponseWithCode(c, statusCode, CodeFromStatus(statusCode), message)
}

// ErrorResponseWithCode sends an error response with a custom code.
func ErrorResponseWithCode(c *gin.Context, statusCode int, code string, message string) {
	resp := Response{
		Code:    code,
		Message: message,
	}
	c.JSON(statusCode, resp)
}

// OK sends 200 OK response
func OK(c *gin.Context, message string, data interface{}) {
	SuccessResponse(c, http.StatusOK, message, data)
}

// OKWithMetadata sends 200 OK response with pagination metadata extracted from PaginatedResponse
func OKWithMetadata(c *gin.Context, message string, paginated PaginatedResponseInterface) {
	c.JSON(http.StatusOK, gin.H{
		"code":     CodeOK,
		"message":  message,
		"data":     paginated.GetData(),
		"metadata": paginated.GetMetadata(),
	})
}

// Created sends 201 Created response
func Created(c *gin.Context, message string, data interface{}) {
	SuccessResponse(c, http.StatusCreated, message, data)
}

// BadRequest sends 400 Bad Request response
func BadRequest(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusBadRequest, message)
}

// Unauthorized sends 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusUnauthorized, message)
}

// Forbidden sends 403 Forbidden response
func Forbidden(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusForbidden, message)
}

// NotFound sends 404 Not Found response
func NotFound(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusNotFound, message)
}

// InternalServerError sends 500 Internal Server Error response
func InternalServerError(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusInternalServerError, message)
}

// ValidationResponse sends 422 Unprocessable Entity response with pre-formatted errors map.
func ValidationResponse(c *gin.Context, errorsMap map[string][]string) {
	resp := Response{
		Code:    CodeValidationError,
		Message: "The given data was invalid.",
		Errors:  errorsMap,
	}
	c.JSON(http.StatusUnprocessableEntity, resp)
}

// ValidationError sends 422 Unprocessable Entity response for a single field error.
func ValidationError(c *gin.Context, field string, message string) {
	ValidationResponse(c, map[string][]string{
		field: {message},
	})
}

// HandleError handles service errors and sends appropriate HTTP response.
// Returns true if error was handled, false if err is nil.
func HandleError(c *gin.Context, err error, fallbackMsg string) bool {
	if err == nil {
		return false
	}

	if fe, ok := err.(*FieldError); ok {
		ValidationError(c, fe.Field, fe.Message)
		return true
	}

	if ce, ok := err.(*CustomError); ok {
		status := ce.Status
		if status == 0 {
			status = http.StatusBadRequest
		}

		code := ce.Code
		if code == "" {
			code = CodeFromStatus(status)
		}

		ErrorResponseWithCode(c, status, code, ce.Message)
		return true
	}

	switch err {
	case ErrNotFound:
		NotFound(c, "Data not found")
	case ErrForbidden:
		Forbidden(c, "Forbidden")
	case ErrInvalidToken, ErrExpiredToken, ErrInvalidCredential, ErrTokenExpired, ErrTokenUsed, ErrTokenInvalid:
		Unauthorized(c, err.Error())
	default:
		InternalServerError(c, fallbackMsg)
	}
	return true
}
