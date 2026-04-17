package main

import (
	"fmt"
	"net/http"

	task_v1 "github.com/fedotovmax/spec-first-openapi/pkg/openapi/task/v1"
	"github.com/go-chi/chi/v5"
)

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

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("--> Чек: авторизация")
		next.ServeHTTP(w, r)
	})
}

type Server struct{}

func (s *Server) GetTaskByID(w http.ResponseWriter, r *http.Request, id task_v1.Id) {

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf("task id is: %s", id)
		fmt.Println("-----> GetTaskByID обработчик")
		w.Write([]byte(response))
	})

	Chain(handler, AuthMiddleware).ServeHTTP(w, r)
}

func main() {
	mux := chi.NewRouter()

	mux.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("-> Global middleware!")
			h.ServeHTTP(w, r)
		})
	})

	taskV1 := &Server{}

	task_v1.HandlerFromMux(taskV1, mux)

	http.ListenAndServe(":8080", mux)
}
