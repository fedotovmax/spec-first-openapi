package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"time"

	"github.com/fedotovmax/spec-first-openapi/domain"
	api_v1 "github.com/fedotovmax/spec-first-openapi/pkg/openapi/api/v1"
	"github.com/fedotovmax/spec-first-openapi/pkg/openapi/operations_v1"
	"github.com/fedotovmax/spec-first-openapi/pkg/openapi/swagger"
	"github.com/fedotovmax/spec-first-openapi/transport/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
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

// Реестр доступных middleware
var middlewareRegistry = map[string]Middleware{
	operations_v1.MwAuth:  AuthMiddleware,
	operations_v1.MwAdmin: AdminMiddleware,
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

func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("-> AdminMiddleware")
		next.ServeHTTP(w, r)
	})
}

func NewOperationIDMiddleware(opsToPattern map[string]string, ops map[string]RouteSettings) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := chi.RouteContext(r.Context())
			pattern := rctx.RoutePattern()
			key := fmt.Sprintf("%s %s", strings.ToUpper(r.Method), pattern)

			if opID, ok := opsToPattern[key]; ok {
				if settings, exists := ops[opID]; exists && len(settings.Middlewares) > 0 {
					Chain(next, settings.Middlewares...).ServeHTTP(w, r)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type Server struct{}

func (h *Server) GetTaskByID(w http.ResponseWriter, r *http.Request, id api_v1.Id) {
	rh := response.NewHTTPResponseHandler(w)

	t := api_v1.Task{
		CreatedAt: time.Now(),
		Id:        uuid.New(),
		IsActive:  true,
		Status:    api_v1.Completed,
		Title:     "New Task 1",
	}

	rh.JSON(t, http.StatusOK)
}

func (h *Server) PatchTaskByID(w http.ResponseWriter, r *http.Request, id api_v1.Id) {

	dto := api_v1.UpdateTask{}

	err := json.NewDecoder(r.Body).Decode(&dto)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	domainUpdate := domain.UpdateTask{
		Title: MapNullable(dto.Title, func(e string) string {
			return string(e)
		}),
		Email: string(dto.Email),
		Status: MapPtr(dto.Status, func(s api_v1.Status) string {
			return string(s)
		}),
	}

	b, _ := json.MarshalIndent(domainUpdate, "", "  ")
	fmt.Println(string(b))
	w.Write([]byte("OK!!!"))
}

func main() {
	mux := chi.NewRouter()

	mux.Use(
		GlobalMiddleware,
	)

	apiImplV1 := &Server{}

	opsv1 := make(map[string]RouteSettings)
	for opID, mwNames := range operations_v1.OperationMiddlewares {
		settings := RouteSettings{}
		for _, name := range mwNames {
			if mw, ok := middlewareRegistry[name]; ok {
				settings.Middlewares = append(settings.Middlewares, mw)
			}
		}
		opsv1[opID] = settings
	}

	operationIDMiddlewareV1 := NewOperationIDMiddleware(operations_v1.PathToOperationID, opsv1)

	apiOptionsV1 := api_v1.ChiServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			rh := response.NewHTTPResponseHandler(w)
			rh.JSON(api_v1.Error{Message: err.Error()}, http.StatusBadRequest)
		},
		Middlewares: []api_v1.MiddlewareFunc{
			func(next http.Handler) http.Handler {
				return operationIDMiddlewareV1(next)
			},
		},
	}

	v1Handler := api_v1.HandlerWithOptions(apiImplV1, apiOptionsV1)

	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Home page"))
	})

	mux.Mount("/", v1Handler)

	mux.Mount("/swagger", swagger.Handler())

	port, err := strconv.Atoi(os.Getenv("PORT"))

	if err != nil {
		panic("port not in env or invalid format")
	}

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
