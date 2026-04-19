# Генерация Openapi с oapi-codegen, redocly

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

output-options:
  nullable-type: true
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

### Operation ID Tool

Помогает извлекать ID операций и сопостовляет их с методом и маршрутом запроса

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
)

// Шаблон генерирует константы и мапу для сопоставления роутов с ID
const tpl = `// Code generated by op-id-gen; DO NOT EDIT.
package {{.Package}}

const (
{{- range .IDs}}
	{{.}} = "{{.}}"
{{- end}}
)

// PathToOperationID мапит "METHOD Path" на OperationID
// Пример ключа: "GET /api/v1/tasks/{id}"
var PathToOperationID = map[string]string{
{{- range $key, $id := .PathMap}}
	"{{$key}}": {{$id}},
{{- end}}
}
`

func main() {
	specPath := flag.String("spec", "", "путь к собранному swagger файлу")
	outputPath := flag.String("out", "", "путь к выходному файлу")
	// Добавим префикс, так как в коде обычно используется r.Mount("/api/v1", ...)
	pathPrefix := flag.String("prefix", "", "префикс пути (например, /api/v1)")
	flag.Parse()

	if *specPath == "" || *outputPath == "" {
		fmt.Println("Использование: go run main.go -spec <path> -out <path> [-prefix /api/v1]")
		os.Exit(1)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(*specPath)
	if err != nil {
		log.Fatalf("Ошибка загрузки спецификации: %v", err)
	}

	uniqueIDs := make(map[string]struct{})
	// Карта: "METHOD /path" -> OperationID
	pathMap := make(map[string]string)

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if op.OperationID != "" {
				uniqueIDs[op.OperationID] = struct{}{}

				// Чистим метод (делаем его UPPERCASE) и склеиваем с префиксом
				fullPath := *pathPrefix + path
				key := fmt.Sprintf("%s %s", strings.ToUpper(method), fullPath)

				pathMap[key] = op.OperationID
			}
		}
	}

	// Сортируем ID для стабильной генерации (чтобы файл не менялся при каждом запуске)
	ids := make([]string, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Определяем имя пакета по папке назначения
	pkgName := filepath.Base(filepath.Dir(*outputPath))

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		log.Fatalf("Ошибка создания директорий: %v", err)
	}

	f, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("Ошибка создания файла: %v", err)
	}
	defer f.Close()

	data := struct {
		Package string
		IDs     []string
		PathMap map[string]string
	}{
		Package: pkgName,
		IDs:     ids,
		PathMap: pathMap,
	}

	t := template.Must(template.New("ids").Parse(tpl))
	if err := t.Execute(f, data); err != nil {
		log.Fatalf("Ошибка выполнения шаблона: %v", err)
	}

	fmt.Printf("✅ Файл сгенерирован: %s (найдено %d операций)\n", *outputPath, len(ids))
}

```
