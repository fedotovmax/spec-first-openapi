package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/fedotovmax/spec-first-openapi/domain"
	api_v1 "github.com/fedotovmax/spec-first-openapi/pkg/openapi/api/v1"
	"github.com/fedotovmax/spec-first-openapi/pkg/openapi/operations"
	"github.com/fedotovmax/spec-first-openapi/transport/http/response"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/strictmiddleware/nethttp"
)

type reqCtx struct{}

var reqCtxKeyValue = reqCtx{}

func RequextFromContext(ctx context.Context) *http.Request {
	req, ok := ctx.Value(reqCtxKeyValue).(*http.Request)

	if !ok {
		panic("no http request in context")
	}

	return req
}

func ToContext(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(
		ctx,
		reqCtxKeyValue,
		req,
	)
}

func GlobalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("-> GlobalMiddleware")
		next.ServeHTTP(w, r)
	})
}

func RequestContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		fmt.Println("-> RequestContextMiddleware!")

		ctx := ToContext(r.Context(), r)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type RouteSettings struct {
	NeedAuth bool
}

func NewStrictAuthMiddleware(ops map[string]RouteSettings) func(nethttp.StrictHTTPHandlerFunc, string) nethttp.StrictHTTPHandlerFunc {
	return func(f nethttp.StrictHTTPHandlerFunc, operationID string) nethttp.StrictHTTPHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req interface{}) (res interface{}, err error) {
			if ops[operationID].NeedAuth {
				fmt.Println("🛡️ Проверка доступа для:", operationID)
				rh := response.NewHTTPResponseHandler(w)

				rh.JSON(api_v1.Error{Message: "Unauthorized"}, http.StatusUnauthorized)
				return nil, nil
			}

			return f(ctx, w, r, req)

		}
	}
}

type Server struct {
}

func (s *Server) GetTaskByID(ctx context.Context, request api_v1.GetTaskByIDRequestObject) (api_v1.GetTaskByIDResponseObject, error) {

	taskID, err := uuid.Parse(request.Id)

	if err != nil {
		return api_v1.GetTaskByID400JSONResponse{
			Errors:  nil,
			Message: err.Error(),
		}, nil
	}

	httpReq := RequextFromContext(ctx)
	fmt.Println("Request url", httpReq.URL)
	fmt.Println("Request ip", httpReq.RemoteAddr)
	fmt.Println("Request agent", httpReq.Header.Get("User-Agent"))
	fmt.Println("Get task request id", taskID)
	return api_v1.GetTaskByID200JSONResponse{Id: taskID.String()}, nil
}

type ServiceInput struct {
	Title domain.Nullable[string]
	ID    uuid.UUID
}

type PatchTaskRequest api_v1.UpdateTask

func (s *Server) PatchTaskByID(ctx context.Context, request api_v1.PatchTaskByIDRequestObject) (api_v1.PatchTaskByIDResponseObject, error) {

	taskID, err := uuid.Parse(request.Id)

	if err != nil {
		return api_v1.PatchTaskByID400JSONResponse{
			Errors:  nil,
			Message: err.Error(),
		}, nil
	}

	fmt.Println("Patch Request id", taskID)

	if request.Body.Status != nil {
		if request.Body.Status.Valid() {
			fmt.Println("Status правильный")
		} else {
			fmt.Println("Status НЕправильный")
		}
	}

	in := ServiceInput{
		Title: request.Body.Title.ToDomain(),
		ID:    taskID,
	}

	_ = in

	return api_v1.PatchTaskByID200JSONResponse{Ok: true}, nil
}

func (r *PatchTaskRequest) Validate() error {

	return nil
}

func main() {
	mux := chi.NewRouter()

	mux.Use(
		GlobalMiddleware,
		RequestContextMiddleware,
	)

	apiV1 := &Server{}

	ops := map[string]RouteSettings{
		operations.GetTaskByID:   {NeedAuth: true},
		operations.PatchTaskByID: {NeedAuth: false},
	}

	strictAuthMiddleware := NewStrictAuthMiddleware(ops)

	v1StrictHandler := api_v1.NewStrictHandler(apiV1, []api_v1.StrictMiddlewareFunc{
		strictAuthMiddleware,
	})

	v1Handler := api_v1.Handler(v1StrictHandler)

	mux.Mount("/api/v1", v1Handler)

	port, err := strconv.Atoi(os.Getenv("PORT"))

	if err != nil {
		panic("port not in env or invalid format")
	}

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
