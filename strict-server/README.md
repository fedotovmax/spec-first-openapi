# Генерация Strict Server Openapi с oapi-codegen, redocly

[Спецификация официальная Official](https://swagger.io/docs/specification/v3_0/about)

[Неофициальная спецификация](https://learn.openapis.org/)

[Redocly CLI](https://github.com/Redocly/redocly-cli)

[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen/)

## Пример

### oapi-codegen.yaml

```yaml
package: api_v1
output: pkg/openapi/api/v1/api.gen.go
generate:
  chi-server: true
  models: true
  strict-server: true

output-options:
  nullable-type: false
  type-mapping:
    string: # Указываем базовый тип OpenAPI
      formats: # Секция для маппинга форматов этого типа
        email:
          type: string
        ipv4:
          type: string
        uuid:
          type: string

import-mapping:
  request: github.com/fedotovmax/spec-first-openapi/transport/http/request
```

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
  OPERATION_ID_TOOL: "{{.PROJECT_GOBIN}}/operation-id"

  GO_API_APP_ENTRYPOINT: "{{.ROOT_DIR}}/cmd/api/main.go"
  OPERATION_ID_TOOL_PATH: "{{.ROOT_DIR}}/cmd/tools/operation-id"

  OPERATION_ID_TOOL_OUTPUT: "{{.ROOT_DIR}}/pkg/openapi/operations/operations.go"

  OPEN_API_V1_SOURCE_FILE: "{{.ROOT_DIR}}/openapi/src/v1/v1.openapi.yaml"
  OPEN_API_V1_BUNDLE: "{{.ROOT_DIR}}/openapi/bundles/v1.openapi.bundle.yaml"

tasks:
  redocly-cli:install:
    silent: true
    desc: Установить локально Redocly CLI
    status:
      - "[ -f {{.REDOCLY}} ]"
    cmds:
      - echo "📦 Устанавливаем redocly-cli..."
      - npm ci

  redocly-v1:bundle:
    silent: true
    desc: Собрать OpenAPI v1 в один файл через локальный redocly
    deps: [redocly-cli:install]
    cmds:
      - echo "🛠️ Сборка OpenApi спецификации версии 1..."
      - "{{.REDOCLY}} bundle {{.OPEN_API_V1_SOURCE_FILE}} -o {{.OPEN_API_V1_BUNDLE}}"

  operation-id-tool:install:
    silent: true
    desc: "Скачивает operation-id-tool в папку go/bin проекта"
    status:
      - "[ -f {{.OPERATION_ID_TOOL}} ]"
    cmds:
      - echo "📦 Устанавливаем operation-id-tool..."
      - mkdir -p {{.PROJECT_GOBIN}}
      - GOBIN={{.PROJECT_GOBIN}} go install {{.OPERATION_ID_TOOL_PATH}}

  operation-id-tool:gen:
    silent: true
    desc: "Генерируем ID операций из спецификации в go пакет"
    deps: [operation-id-tool:install]
    cmds:
      - echo "🛠️ Генерация ID операций..."
      - "{{.OPERATION_ID_TOOL}} -spec {{.OPEN_API_V1_BUNDLE}} -out {{.OPERATION_ID_TOOL_OUTPUT}}"

  oapi-codegen:install:
    silent: true
    desc: "Скачивает oapi-codegen в папку go/bin проекта"
    status:
      - "[ -f {{.OAPI_CODEGEN}} ]"
    cmds:
      - echo "📦 Устанавливаем oapi-codegen..."
      - mkdir -p {{.PROJECT_GOBIN}}
      - GOBIN={{.PROJECT_GOBIN}} go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@{{.OAPI_CODEGEN_VERSION}}

  oapi-codegen-v1:gen:
    silent: true
    desc: "Генерация Go-кода из всех OpenAPI-декларации"
    deps: [oapi-codegen:install]
    cmds:
      - task redocly-v1:bundle
      - echo "🛠️ Генерация через oapi codegen"
      - "{{.OAPI_CODEGEN}} -config oapi-codegen.yaml {{.OPEN_API_V1_BUNDLE}}"
      - task operation-id-tool:gen

  go-dev:run:
    silent: true
    desc: "Запуск go run"
    cmds:
      - echo "🏁 Запуск go приложения"
      - go run {{.GO_API_APP_ENTRYPOINT}}
```

### main.go

```go
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
```

### domain.Nullable

```go
package domain

type Nullable[T any] struct {
	Value *T
	Set   bool
}
```

### request.Nullable

```go
package request

import (
	"encoding/json"

	"github.com/fedotovmax/spec-first-openapi/domain"
)

type Nullable[T any] struct {
	domain.Nullable[T]
}

func (n *Nullable[T]) UnmarshalJSON(b []byte) error {
	n.Set = true

	if string(b) == "null" {
		n.Value = nil
		return nil
	}

	var value T

	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}

	n.Value = &value

	return nil
}

func (n *Nullable[T]) ToDomain() domain.Nullable[T] {
	return n.Nullable
}
```

### Operation ID tool

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
)

// Шаблон генерации: создает Go-файл с константами OperationID
const tpl = `// Code generated by op-id-gen; DO NOT EDIT.
package {{.Package}}

const (
{{- range .IDs}}
	{{.}} = "{{.}}"
{{- end}}
)
`

func main() {
	// 1. Инициализация флагов командной строки
	specPath := flag.String("spec", "", "путь к собранному swagger файлу")
	outputPath := flag.String("out", "", "путь к выходному файлу")
	flag.Parse()

	if *specPath == "" || *outputPath == "" {
		fmt.Println("Использование: go run tools/gen-op-ids/main.go -spec <path> -out <path>")
		os.Exit(1)
	}

	// 2. Загрузка и парсинг OpenAPI спецификации
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(*specPath)
	if err != nil {
		log.Fatalf("Ошибка загрузки спецификации: %v", err)
	}

	// 3. Сбор уникальных OperationID из всех путей и методов
	uniqueIDs := make(map[string]struct{})
	for _, pathItem := range doc.Paths.Map() {
		for _, op := range pathItem.Operations() {
			if op.OperationID != "" {
				uniqueIDs[op.OperationID] = struct{}{}
			}
		}
	}

	// Сортировка для детерминированного вывода (чтобы git diff был чистым)
	ids := make([]string, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// 4. Подготовка файловой структуры
	dir := filepath.Dir(*outputPath)
	pkgName := filepath.Base(dir)

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Ошибка создания директорий: %v", err)
	}

	f, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("Ошибка создания файла: %v", err)
	}
	defer f.Close()

	// 5. Рендеринг шаблона в файл
	data := struct {
		Package string
		IDs     []string
	}{
		Package: pkgName,
		IDs:     ids,
	}

	t := template.Must(template.New("ids").Parse(tpl))
	if err := t.Execute(f, data); err != nil {
		log.Fatalf("Ошибка выполнения шаблона: %v", err)
	}

	fmt.Printf("✅ Константы OperationID созданы: %s (пакет: %s)\n", *outputPath, pkgName)
}
```
