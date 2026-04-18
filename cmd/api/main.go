package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/fedotovmax/spec-first-openapi/domain"
	api_v1 "github.com/fedotovmax/spec-first-openapi/pkg/openapi/api/v1"
	"github.com/fedotovmax/spec-first-openapi/transport/http/response"
	"github.com/go-chi/chi/v5"
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

func RequestContext(f nethttp.StrictHTTPHandlerFunc, operationID string) nethttp.StrictHTTPHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (response interface{}, err error) {
		fmt.Println("--> RequestContext middleware")
		ctx = ToContext(ctx, r)
		return f(ctx, w, r, request)
	}
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("--> Чек: авторизация")
		next.ServeHTTP(w, r)
	})
}

type Server struct {
}

func (s *Server) GetTaskByID(ctx context.Context, request api_v1.GetTaskByIDRequestObject) (api_v1.GetTaskByIDResponseObject, error) {
	httpReq := RequextFromContext(ctx)
	fmt.Println("Request url", httpReq.URL)
	fmt.Println("Request ip", httpReq.RemoteAddr)
	fmt.Println("Request agent", httpReq.Header.Get("User-Agent"))
	fmt.Println(request.Id)
	return nil, nil
}

type ServiceInput struct {
	Title domain.Nullable[string]
}

type PatchTaskRequest api_v1.UpdateTask

func (r *PatchTaskRequest) Validate() error {

	return nil
}

func (s *Server) PatchTaskByID(ctx context.Context, request api_v1.PatchTaskByIDRequestObject) (api_v1.PatchTaskByIDResponseObject, error) {

	fmt.Println(request.Body.Title.Set)

	return nil, nil
}

func main() {
	mux := chi.NewRouter()

	mux.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("-> Global middleware!")
			h.ServeHTTP(w, r)
		})
	})

	apiV1 := &Server{}

	v1StrictOptions := api_v1.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  func(w http.ResponseWriter, r *http.Request, err error) {},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {},
	}

	v1StrictHandler := api_v1.NewStrictHandlerWithOptions(
		apiV1,
		[]api_v1.StrictMiddlewareFunc{
			RequestContext,
		},
		v1StrictOptions,
	)

	v1ChiOptions := api_v1.ChiServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			rh := response.NewHTTPResponseHandler(w)
			rh.JSON(api_v1.Error{Message: err.Error()}, http.StatusBadRequest)
		},
	}

	v1Handler := api_v1.HandlerWithOptions(v1StrictHandler, v1ChiOptions)

	mux.Mount("/api/v1", v1Handler)

	port, err := strconv.Atoi(os.Getenv("PORT"))

	if err != nil {
		panic("port not in env or invalid format")
	}

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
