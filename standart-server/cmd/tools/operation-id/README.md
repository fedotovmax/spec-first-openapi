## 1. Загрузка спецификации (Parsing)

Тулза не читает файл просто как текст, она использует openapi3.NewLoader(), чтобы превратить YAML в дерево объектов Go.

Go

```go
loader := openapi3.NewLoader()
doc, err := loader.LoadFromFile(\*specPath)
if err != nil {
  log.Fatalf("Ошибка загрузки: %v", err)
}
```

Здесь doc становится «картой» твоего API, где можно достать любой путь, метод или кастомное поле.

## 2. Извлечение данных (The Crawler)

Это сердце программы. Мы бежим по всем путям спецификации и собираем три типа данных:

Имена операций (operationId).

Связку «Метод + Путь».

Твои кастомные middleware из x-operation-middlewares.

Go

```go
for path, pathItem := range doc.Paths.Map() {
  for method, op := range pathItem.Operations() {
    if op.OperationID != "" {
    // 1. Сохраняем ID операции для создания констант
    uniqueIDs[op.OperationID] = struct{}{}

          // 2. Делаем ключ "GET /tasks/{id}" для маппинга в роутере
          key := fmt.Sprintf("%s %s", strings.ToUpper(method), path)
          pathMap[key] = op.OperationID

          // 3. Достаем наш кастомный массив middleware
          if ext, ok := op.Extensions["x-operation-middlewares"]; ok {
            if mws, ok := ext.([]interface{}); ok {
              for _, mw := range mws {
                if s, ok := mw.(string); ok {
                  uniqueMws[s] = struct{}{} // Собираем уникальные имена для констант
                  opMiddlewares[op.OperationID] = append(opMiddlewares[op.OperationID], s)
            }
          }
        }
      }
    }
  }
}
```

## 3. Подготовка имен и констант (Naming)

Чтобы имена из YAML превратились в красивые константы Go (например, auth -> MwAuth), используется функция title. А чтобы файл не менялся хаотично при каждой генерации, данные сортируются.

Go

```go
func title(s string) string {

  if s == "" { return "" }

  return strings.ToUpper(s[:1]) + s[1:] // Делает первую букву заглавной
}

// ... в main ...
for \_, m := range mws {
  mwConsts = append(mwConsts, MwConst{
    Name: mwPrefix + title(m), // Получаем MwAuth, MwAdmin и т.д.
    Value: m, // Оригинальное строковое значение "auth"
  })
}
```

## 4. Генерация файла (Templating)

В конце тулза берет текстовый шаблон (tpl) и «вклеивает» туда собранные данные. Пакет определяется автоматически по имени папки:

Go

```go
data := struct {
  Package string
// ... поля данных ...
}{
  Package: filepath.Base(filepath.Dir(\*outputPath)), // Берет "v1" из пути "pkg/.../v1/..."
// ...
}

t := template.Must(template.New("ids").Funcs(funcMap).Parse(tpl))
t.Execute(f, data)
```

Сам шаблон использует цикл range, чтобы выплюнуть строки кода в итоговый файл:

```go
Go
const (
  {{- range .IDs}}
  {{.}} = "{{.}}" // Генерирует GetTaskByID = "GetTaskByID"
  {{- end}}
)
```

## Итоговый результат работы

- На выходе получается файл, который связывает мир строк (HTTP запросы и YAML) с миром типов Go.

- PathToOperationID говорит: «Для этого запроса используй этот ID».

- OperationMiddlewares говорит: «Для этого ID примени вот эти константы middleware».

- Middleware Registry (в твоем main.go) говорит: «Для этой константы используй вот эту функцию в коде».

- Цепочка замыкается, и у тебя получается полностью автоматизированный и безопасный пайплайн.
