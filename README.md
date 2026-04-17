# Генерация Openapi с oapi-codegen, redocly

[Спецификация официальная Official](https://swagger.io/docs/specification/v3_0/about)

[Неофициальная спецификация](https://learn.openapis.org/)

[Redocly CLI](https://github.com/Redocly/redocly-cli)

[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen/)

## Пример

### Taskfile.yaml

```yaml
version: "3"

dotenv: [".env"]

vars:
  NODE_MODULES_BIN: "{{.ROOT_DIR}}/node_modules/.bin"
  REDOCLY: "{{.NODE_MODULES_BIN}}/redocly"

  OAPI_CODEGEN_VERSION: "v2.6.0"

  PROJECT_GOBIN: "{{.ROOT_DIR}}/go/bin"
  OAPI_CODEGEN: "{{.PROJECT_GOBIN}}/oapi-codegen"

  OPEN_API_TASKS_V1_SOURCE_FILE: "{{.ROOT_DIR}}/openapi/src/tasks/v1/tasks.openapi.yaml"
  OPEN_API_TASK_V1_BUNDLE: "{{.ROOT_DIR}}/openapi/bundles/tasks.openapi.v1.bundle.yaml"

tasks:
  redocly-cli:install:
    desc: Установить локально Redocly CLI
    status:
      - "[ -f {{.REDOCLY}} ]"
    cmds:
      - echo "📦 Устанавливаем redocly-cli..."
      - npm ci

  redocly-task-v1:bundle:
    desc: Собрать OpenAPI Tasks v1 в один файл через локальный redocly
    deps: [redocly-cli:install]
    cmds:
      - "{{.REDOCLY}} bundle {{.OPEN_API_TASKS_V1_SOURCE_FILE}} -o {{.OPEN_API_TASK_V1_BUNDLE}}"

  oapi-codegen:install:
    desc: "Скачивает oapi-codegen в папку go/bin проекта"
    status:
      - "[ -f {{.OAPI_CODEGEN}} ]"
    cmds:
      - echo "📦 Устанавливаем oapi-codegen..."
      - mkdir -p {{.PROJECT_GOBIN}}
      - GOBIN={{.PROJECT_GOBIN}} go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@{{.OAPI_CODEGEN_VERSION}}

  oapi-codegen-task-v1:gen:
    desc: "Генерация Go-кода из всех OpenAPI-декларации"
    deps: [oapi-codegen:install]
    cmds:
      - task redocly-task-v1:bundle
      - "{{.OAPI_CODEGEN}} -o pkg/openapi/task/v1/task_api.gen.go -package task_v1 -generate chi-server,strict-server,types {{.OPEN_API_TASK_V1_BUNDLE}}"
```

### main.go

```go
package main

import (
	"fmt"
	"net/http"

	task_v1 "github.com/fedotovmax/code-first-openapi/pkg/openapi/task/v1"
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

```
