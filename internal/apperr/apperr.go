package apperr

import "fmt"

type Error struct {
	Status  int    `json:"status"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("%s: %s", e.Title, e.Message)
	}
	return fmt.Sprintf("%s (%s): %s", e.Title, e.Code, e.Message)
}

func New(status int, title, message, code string) *Error {
	return &Error{Status: status, Title: title, Message: message, Code: code}
}

func InvalidRequest(message string) *Error {
	return New(400, "Invalid request", message, "INVALID_REQUEST")
}

func Unauthorized(message string) *Error {
	return New(401, "Unauthorized", message, "UNAUTHORIZED")
}

func NotFound() *Error {
	return New(404, "Not found", "The requested resource was not found.", "NOT_FOUND")
}

func Conflict(title, message string) *Error {
	return New(409, title, message, "CONFLICT")
}

func UnprocessableEntity(title, message, code string) *Error {
	return New(422, title, message, code)
}

func TooManyRequests() *Error {
	return New(429, "Too many attempts", "Please wait a moment and try again.", "RATE_LIMITED")
}

func InternalServerError() *Error {
	return New(500, "Something went wrong", "The server could not complete this request. Nothing was changed.", "INTERNAL_ERROR")
}
