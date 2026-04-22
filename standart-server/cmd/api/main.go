package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/fedotovmax/spec-first-openapi/domain"
	api_v1 "github.com/fedotovmax/spec-first-openapi/pkg/openapi/api/v1"
	"github.com/fedotovmax/spec-first-openapi/pkg/openapi/operations"
	"github.com/fedotovmax/spec-first-openapi/transport/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/oapi-codegen/nullable"
	"github.com/oapi-codegen/runtime/types"
)

func MapNullable[T any, R any](src nullable.Nullable[T], transform func(T) R) domain.Nullable[R] {

	if !src.IsSpecified() {
		return domain.Nullable[R]{Set: false}
	}

	if src.IsNull() {
		return domain.Nullable[R]{Set: true, Value: nil}
	}

	// Извлекаем значение из транспортной мапы
	val, _ := src.Get()

	// Трансформируем его (например, Email -> string)
	res := transform(val)

	return domain.Nullable[R]{
		Set:   true,
		Value: &res,
	}
}

func MapPtr[T any, R any](src *T, transform func(T) R) *R {
	if src == nil {
		return nil
	}

	res := transform(*src)
	return &res
}

type Middleware = func(http.Handler) http.Handler

func Chain(
	h http.Handler,
	m ...Middleware,
) http.Handler {

	if len(m) == 0 {
		return h
	}

	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}

	return h
}

type RouteSettings struct {
	Middlewares []Middleware
}

func GlobalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("-> GlobalMiddleware")
		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("-> AuthMiddleware")
		next.ServeHTTP(w, r)
	})
}

func NewOperationIDMiddleware(prefix string, ops map[string]RouteSettings) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := chi.RouteContext(r.Context())
			// Находим чистый путь для мапы
			pattern := rctx.RoutePattern()
			externalPath := strings.TrimPrefix(pattern, prefix)
			key := fmt.Sprintf("%s %s", strings.ToUpper(r.Method), externalPath)

			// Проверяем, есть ли настройки для этого OperationID
			if opID, ok := operations.PathToOperationID[key]; ok {
				fmt.Println("OP", opID)
				if settings, exists := ops[opID]; exists && len(settings.Middlewares) > 0 {
					// Используем твой Chain, чтобы обернуть "next" (твой хендлер сервера)
					// в специфичные для этого роута middleware
					wrappedHandler := Chain(next, settings.Middlewares...)
					wrappedHandler.ServeHTTP(w, r)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

type Server struct{}

func (h *Server) GetTaskByID(w http.ResponseWriter, r *http.Request, id api_v1.Id) {

}

func (h *Server) PatchTaskByID(w http.ResponseWriter, r *http.Request, id api_v1.Id) {

	dto := api_v1.UpdateTask{}

	err := json.NewDecoder(r.Body).Decode(&dto)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	domainUpdate := domain.UpdateTask{
		Title: MapNullable(dto.Title, func(e types.Email) string {
			return string(e)
		}),
		Email: string(dto.Email),
		Status: MapPtr(dto.Status, func(s api_v1.Status) string {
			return string(s)
		}),
	}

	b, _ := json.MarshalIndent(domainUpdate, "", "  ")
	fmt.Println(string(b))
}

func main() {
	mux := chi.NewRouter()

	mux.Use(
		GlobalMiddleware,
	)

	apiImpl := &Server{}

	v1Prefix := "/api/v1"

	ops := map[string]RouteSettings{
		operations.GetTaskByID: {
			Middlewares: []Middleware{
				AuthMiddleware,
			},
		},
	}

	operationIDMiddleware := NewOperationIDMiddleware(v1Prefix, ops)

	apiOptions := api_v1.ChiServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			rh := response.NewHTTPResponseHandler(w)
			rh.JSON(api_v1.Error{Message: err.Error()}, http.StatusBadRequest)
		},
		Middlewares: []api_v1.MiddlewareFunc{
			func(next http.Handler) http.Handler {
				return operationIDMiddleware(next)
			},
		},
	}

	v1Handler := api_v1.HandlerWithOptions(apiImpl, apiOptions)

	mux.Mount(v1Prefix, v1Handler)

	port, err := strconv.Atoi(os.Getenv("PORT"))

	if err != nil {
		panic("port not in env or invalid format")
	}

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
