# swagger-ui-gen

Инструмент кодогенерации для встраивания Swagger UI в Go-сервер.

Сканирует папку с OpenAPI бандлами, встраивает их через `//go:embed` и генерирует `http.Handler` с роутами для каждой версии API.

## Использование

```bash
swagger-ui -bundles ./openapi/bundles -out ./pkg/swagger/swagger.gen.go
```

### Флаги

| Флаг       | Обязательный | Описание                                                         |
| ---------- | ------------ | ---------------------------------------------------------------- |
| `-bundles` | да           | Путь к папке с бандлами                                          |
| `-out`     | да           | Путь к выходному `.go` файлу                                     |
| `-package` | нет          | Имя пакета (по умолчанию берётся из имени папки выходного файла) |

## Формат бандлов

Тул ищет файлы по паттерну `{версия}.openapi.bundle.json` и ожидает рядом соответствующий `{версия}.openapi.bundle.yaml`.

Пример:

```
openapi/bundles/
  v1.openapi.bundle.json
  v1.openapi.bundle.yaml
  v2.openapi.bundle.json
  v2.openapi.bundle.yaml
```

## Что генерируется

В выходном файле создаётся функция `Handler() http.Handler` и папка `bundles/` рядом с ним с копиями бандлов для `//go:embed`.

Роуты для каждой версии:

| Путь                  | Описание                     |
| --------------------- | ---------------------------- |
| `/{версия}/spec.json` | OpenAPI спека в JSON         |
| `/{версия}/spec.yaml` | OpenAPI спека в YAML         |
| `/{версия}/*`         | Swagger UI                   |
| `/`                   | Редирект на UI первой версии |

## Подключение в приложении

```go
import "yourmodule/pkg/swagger"

mux.Mount("/swagger", swagger.Handler())
```

После этого Swagger UI доступен по адресу `http://localhost:8080/swagger/v1/index.html`.

## Интеграция с Taskfile

```yaml
vars:
  SWAGGER_UI_TOOL: "{{.PROJECT_GOBIN}}/swagger-ui"
  SWAGGER_UI_TOOL_PATH: "{{.ROOT_DIR}}/cmd/tools/swagger-ui"
  SWAGGER_OUTPUT: "{{.ROOT_DIR}}/pkg/swagger/swagger.gen.go"

tasks:
  swagger-ui-tool:install:
    silent: true
    status:
      - "[ -f {{.SWAGGER_UI_TOOL}} ]"
    cmds:
      - mkdir -p {{.PROJECT_GOBIN}}
      - GOBIN={{.PROJECT_GOBIN}} go install {{.SWAGGER_UI_TOOL_PATH}}

  swagger:gen:
    silent: true
    deps: [swagger-ui-tool:install]
    cmds:
      - echo "📖 Генерация Swagger UI handler..."
      - "{{.SWAGGER_UI_TOOL}} -bundles {{.ROOT_DIR}}/openapi/bundles -out {{.SWAGGER_OUTPUT}}"
```

## .gitignore

Генерируемые файлы не нужно коммитить:

```gitignore
pkg/swagger/bundles/
pkg/swagger/swagger.gen.go
```
