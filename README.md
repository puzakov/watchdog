# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки

Бенчмарки измеряют скорость ключевых компонентов: хранилище метрик, HTTP-обработчики, middleware (gzip, SHA256) и подпись запросов.

```bash
# все бенчмарки с метриками памяти
go test -bench=. -benchmem ./internal/service/ ./internal/handler/ ./internal/middleware/ ./internal/sign/

# отдельные компоненты
go test -bench=BenchmarkMemStorage -benchmem ./internal/service/
go test -bench=BenchmarkHandleUpdates -benchmem ./internal/handler/
go test -bench=BenchmarkGzip -benchmem ./internal/middleware/
go test -bench=BenchmarkSumSHA256 -benchmem ./internal/sign/
```

Пример результатов (Apple M3 Max):

| Бенчмарк | ns/op | B/op | allocs/op |
|----------|-------|------|-----------|
| MemStorage_UpdateBatch | ~1032 | 0 | 0 |
| HandleUpdatesJSON | ~40746 | ~38005 | ~232 |
| HandleIndex | ~7634 | ~14225 | ~83 |
| Gzip | ~1721 | ~10671 | ~22 |
| SumSHA256 | ~98 | 192 | 3 |

## Профилирование памяти

### Снятие профиля под нагрузкой

Бенчмарк `BenchmarkHeapProfile` эмулирует типичную нагрузку: пакетные обновления `/updates` и отображение `/` через gzip + SHA256 middleware.

```bash
# базовый профиль (до оптимизации) — profiles/base.pprof
# результирующий профиль (после оптимизации) — profiles/result.pprof
./scripts/capture_heap.sh profiles/result.pprof

# или вручную
go test -bench=BenchmarkHeapProfile -benchtime=3s -memprofile=profiles/result.pprof ./internal/handler/
```

Сервер также поддерживает pprof-эндпоинт для профилирования под внешней нагрузкой (hey, wrk, ab):

```bash
go run ./cmd/server -pprof localhost:6060

# в другом терминале — нагрузка и снятие heap-профиля
go install github.com/rakyll/hey@latest
hey -z 10s -c 10 -m POST -H "Content-Type: application/json" \
  -d '[{"id":"Alloc","type":"gauge","value":1.5}]' http://localhost:8080/updates
curl -o profiles/runtime.pprof http://localhost:6060/debug/pprof/heap
```

### Анализ профиля

```bash
go tool pprof -top profiles/base.pprof
go tool pprof -list=normalizeBatch profiles/base.pprof
go tool pprof -http=:8081 profiles/base.pprof   # web UI
go tool pprof -peek=HandleUpdatesJSON profiles/base.pprof
```

### Оптимизации

По результатам анализа `profiles/base.pprof` были оптимизированы:

1. **Gzip middleware** — переиспользование `gzip.Writer` через `sync.Pool` вместо создания нового компрессора на каждый ответ.
2. **normalizeBatch** — удалены избыточные map/slice (`order`, `seen`); слияние дубликатов выполняется за один проход.
3. **HandleIndex** — предварительный `Grow` для `strings.Builder`, запись через `io.WriteString`.
4. **SumSHA256** — один буфер + `sha256.Sum256` вместо `sha256.New()` + нескольких аллокаций.
5. **HashSHA256 middleware** — переиспользование `bufferedResponseWriter` через `sync.Pool`.
6. **Agent gzip** — пул буферов и gzip-писателей в `sender.go`.

### Сравнение профилей (до / после)

```bash
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

```
File: handler.test
Type: alloc_space
Time: 2026-06-15 12:55:09 +05
Showing nodes accounting for -45449.19MB, 91.51% of 49663.54MB total
Dropped 115 nodes (cum <= 248.32MB)
      flat  flat%   sum%        cum   cum%
-25702.55MB 51.75% 51.75% -46448.59MB 93.53%  compress/flate.NewWriter (inline)
-12605.93MB 25.38% 77.14% -20746.04MB 41.77%  compress/flate.(*compressor).init
   -7994MB 16.10% 93.23%    -7994MB 16.10%  compress/flate.newDeflateFast (inline)
  487.66MB  0.98% 92.25%   487.66MB  0.98%  reflect.growslice
  394.84MB   0.8% 91.46%   394.84MB   0.8%  encoding/json.(*Decoder).refill
 -280.62MB  0.57% 92.02%  -280.62MB  0.57%  compress/flate.(*huffmanEncoder).generate
  243.92MB  0.49% 91.53%   243.92MB  0.49%  bufio.NewReaderSize (inline)
      -8MB 0.016% 91.55% -23642.46MB 47.61%  github.com/puzakov/watchdog/internal/handler.HandleIndex
       8MB 0.016% 91.53%   240.59MB  0.48%  io.WriteString
    3.50MB 0.007% 91.52% -45763.97MB 92.15%  github.com/puzakov/watchdog/internal/handler.BenchmarkHeapProfile.Gzip.func1
    2.50MB 0.005% 91.52%   262.37MB  0.53%  net/http/httptest.NewRequestWithContext
    1.50MB 0.003% 91.51% -21816.82MB 43.93%  github.com/puzakov/watchdog/internal/handler.HandleUpdatesJSON
         0     0% 91.51%   243.92MB  0.49%  bufio.NewReader (inline)
         0     0% 91.51%  -298.12MB   0.6%  compress/flate.(*Writer).Close (inline)
         0     0% 91.51%  -298.12MB   0.6%  compress/flate.(*compressor).close
         0     0% 91.51%  -286.62MB  0.58%  compress/flate.(*compressor).encSpeed
         0     0% 91.51%  -286.62MB  0.58%  compress/flate.(*huffmanBitWriter).writeBlockDynamic
         0     0% 91.51%  -298.62MB   0.6%  compress/gzip.(*Writer).Close
         0     0% 91.51% -46451.59MB 93.53%  compress/gzip.(*Writer).Write
         0     0% 91.51%   937.99MB  1.89%  encoding/json.(*Decoder).Decode
         0     0% 91.51%   395.34MB   0.8%  encoding/json.(*Decoder).readValue
         0     0% 91.51% -22912.66MB 46.14%  encoding/json.(*Encoder).Encode
         0     0% 91.51%   543.16MB  1.09%  encoding/json.(*decodeState).array
         0     0% 91.51%   542.66MB  1.09%  encoding/json.(*decodeState).unmarshal
         0     0% 91.51%   543.16MB  1.09%  encoding/json.(*decodeState).value
         0     0% 91.51% -21816.82MB 43.93%  github.com/go-chi/chi/v5.(*Mux).Mount.func1
         0     0% 91.51% -45468.85MB 91.55%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 91.51% -45459.29MB 91.53%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 91.51% -45381.06MB 91.38%  github.com/puzakov/watchdog/internal/handler.BenchmarkHeapProfile
         0     0% 91.51%  -298.62MB   0.6%  github.com/puzakov/watchdog/internal/handler.BenchmarkHeapProfile.Gzip.func1.1
         0     0% 91.51% -45675.44MB 91.97%  github.com/puzakov/watchdog/internal/handler.BenchmarkHeapProfile.HashSHA256.func2
         0     0% 91.51% -23642.46MB 47.61%  github.com/puzakov/watchdog/internal/handler.NewHandler.func1
         0     0% 91.51% -21816.82MB 43.93%  github.com/puzakov/watchdog/internal/handler.NewHandler.func5.1
         0     0% 91.51% -45381.06MB 91.38%  github.com/puzakov/watchdog/internal/handler.simulateLoad
         0     0% 91.51% -46451.59MB 93.53%  github.com/puzakov/watchdog/internal/middleware.(*gzipResponseWriter).Write
         0     0% 91.51% -45675.44MB 91.97%  net/http.HandlerFunc.ServeHTTP
         0     0% 91.51%   262.37MB  0.53%  net/http/httptest.NewRequest (inline)
         0     0% 91.51%   487.66MB  0.98%  reflect.Value.Grow
         0     0% 91.51%   487.66MB  0.98%  reflect.Value.grow
         0     0% 91.51% -45378.20MB 91.37%  testing.(*B).launch
         0     0% 91.51% -45381.06MB 91.38%  testing.(*B).runN
```

Отрицательные значения в колонках `flat` и `cum` показывают снижение объёма аллокаций после оптимизации. Наибольший выигрыш — в gzip-сжатии (`compress/flate`, `compress/gzip`): ~93% cumulativе reduction за счёт `sync.Pool`.

Бенчмарк `BenchmarkHeapProfile` до оптимизации: **~2.5 MB/op, 477 allocs/op** → после: **~92 KB/op, 404 allocs/op**.

## Покрытие тестами

```bash
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

Текущее покрытие: **47.8%** (требование спринта — не менее 40%).
