package reuse

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorCode string      `json:"error_code,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func Success(w http.ResponseWriter, msg string, data interface{}) {
	JSON(w, http.StatusOK, Response{Success: true, Message: msg, Data: data})
}

func Created(w http.ResponseWriter, msg string, data interface{}) {
	JSON(w, http.StatusCreated, Response{Success: true, Message: msg, Data: data})
}

func Error(w http.ResponseWriter, status int, err string) {
	if status >= http.StatusInternalServerError {
		err = http.StatusText(status)
		if err == "" {
			err = "Internal Server Error"
		}
	}
	JSON(w, status, Response{Success: false, Error: err})
}

// ErrorWithCode sends a standardized JSON error response with machine-readable error code
func ErrorWithCode(w http.ResponseWriter, status int, code, err string) {
	if status >= http.StatusInternalServerError && err == "" {
		err = http.StatusText(status)
		if err == "" {
			err = "Internal Server Error"
		}
	}
	JSON(w, status, Response{Success: false, Error: err, ErrorCode: code})
}

func Message(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, Response{Success: true, Message: msg})
}
