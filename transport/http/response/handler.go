package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type ErrorResponse struct {
	Message string `json:"message" validate:"required"`
	Error   string `json:"error" validate:"required"`
}

type HTTPResponseHandler struct {
	rw http.ResponseWriter
}

func NewHTTPResponseHandler(rw http.ResponseWriter) *HTTPResponseHandler {
	return &HTTPResponseHandler{
		rw: rw,
	}
}

func (h *HTTPResponseHandler) HandlePanic(p any, msg string) {

	const op = "core.transport.http.response.HTTPResponseHandler.HandlePanic"

	err := fmt.Errorf("%s: unexpected panic: %v", op, p)

	response := ErrorResponse{
		Message: msg,
		Error:   err.Error(),
	}

	h.JSON(response, http.StatusInternalServerError)

}

func (h *HTTPResponseHandler) JSON(body any, statusCode int) {

	h.rw.Header().Set("Content-Type", "application/json")

	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(h.rw, `{"message": "failed to encode json"}`, http.StatusInternalServerError)
		return
	}

	h.rw.WriteHeader(statusCode)
	h.rw.Write(buf.Bytes())
}

func (h *HTTPResponseHandler) NoContent() {
	h.rw.WriteHeader(http.StatusNoContent)
}
