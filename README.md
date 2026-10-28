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

## Сборка и запуск

```bash
# Сборка всех пакетов
go build ./...

# Запуск сервера
go run cmd/server/main.go

# Запуск агента
go run cmd/agent/main.go

# Флаги агента (опционально)
go run cmd/agent/main.go -a localhost:8080 -p 2 -r 10

# Сборка server и agent в bin/ с версией, датой и коммитом
VERSION=v1.0.0 ./scripts/build.sh
```

При старте сервер и агент печатают `Build version`, `Build date` и `Build commit`; без `scripts/build.sh` (например, через `go run`) во всех трёх строках будет `N/A`.

## Тесты

```bash
# Все тесты во всех пакетах
go test ./...

# Тесты конкретного пакета
go test ./internal/agent/...
go test ./internal/handler/...

# С подробным выводом
go test -v ./...
```

## Аудит

Сервер публикует событие после каждого успешного обновления метрик: `{"ts": <unix>, "metrics": [<имена>], "ip_address": "<адрес>"}`. Получатели включаются флагами `--audit-file` / `AUDIT_FILE` (JSON-строки в файл) и `--audit-url` / `AUDIT_URL` (POST на указанный URL).

- Доставка синхронная: события уходят получателям по очереди внутри обработчика запроса. Таймаут HTTP-получателя 2 секунды, поэтому недоступный `AUDIT_URL` замедляет каждое успешное обновление до этого времени. Ошибки получателей пишутся в лог и не влияют на ответ клиенту.
- Адрес клиента берётся из заголовка `X-Real-IP`, а если его нет, то из `RemoteAddr`. Заголовок не аутентифицирован: его должен выставлять обратный прокси, иначе клиент может подставить любой адрес. `X-Forwarded-For` не используется.

## Доверенная подсеть

Флаг `-t` / `TRUSTED_SUBNET` / `trusted_subnet` задаёт подсеть в формате CIDR (например, `192.168.1.0/24`). Сервер принимает запросы на запись метрик (`/update…`, `/updates/`) только с заголовком `X-Real-IP`, адрес в котором входит в эту подсеть. Иначе отвечает `403 Forbidden`. Чтение (`/`, `/value…`, `/ping`) не ограничено. Пустое значение отключает проверку.

- Сервер смотрит только на `X-Real-IP`, без `RemoteAddr`.
- Агент при старте определяет IP, с которого ходит к серверу, и отправляет его в `X-Real-IP` с каждым запросом. Сначала пробуется IPv4, поэтому для `-a localhost:8080` это `127.0.0.1`.
- Заголовок выставляет сам клиент, так что это защита для работы за обратным прокси, а не от подделки адреса.

```bash
go run cmd/server/main.go -t 127.0.0.0/8
```

## Профилирование

### Бенчмарки

```bash
go test -run xxx -bench . -benchmem ./internal/...
```

### Профиль памяти

Нужен `ab` (ApacheBench) и `go tool pprof`. pprof на сервере выключен по умолчанию и включается флагом `--pprof-address` или переменной `PPROF_ADDRESS`. Адрес надо привязывать к `127.0.0.1`: сервер pprof без авторизации, наружу его открывать нельзя.

```bash
./scripts/gen-load-data.sh load   # load/batch.json, load/batch.json.gz, load/value.json

go run ./cmd/server -a 127.0.0.1:8085 -f "" -r=false --pprof-address 127.0.0.1:6065

ab -k -n 30000 -c 50 -p load/batch.json.gz -T application/json -H "Content-Encoding: gzip" http://127.0.0.1:8085/updates/
ab -k -n 30000 -c 50 -p load/value.json -T application/json -H "Accept-Encoding: gzip" http://127.0.0.1:8085/value
ab -k -n 10000 -c 20 -H "Accept-Encoding: gzip" http://127.0.0.1:8085/

curl -o profiles/base.pprof http://127.0.0.1:6065/debug/pprof/allocs
```

В `base.pprof` (`go tool pprof -sample_index=alloc_space -peek 'compress/flate.NewWriter$' profiles/base.pprof`) 92,6% выделенной памяти приходится на `compress/flate.NewWriter` (30784 МБ из 33233 МБ): gzip-писатель создавался заново на каждый ответ, `gzip.Reader` — на каждый запрос. Ещё HTML-шаблон главной страницы разбирался при каждом обращении.

Что изменено:

- `internal/middleware/gzip.go`: `gzip.Writer` и `gzip.Reader` берутся из `sync.Pool`.
- `internal/agent/agent.go`: `gzip.Writer` при отправке батча тоже из `sync.Pool`.
- `internal/handler/update.go`: шаблон главной страницы разбирается один раз.

После изменений нагрузка повторена теми же командами, профиль сохранён в `profiles/result.pprof`:

```bash
go tool pprof -top -nodecount=10 -diff_base=profiles/base.pprof profiles/result.pprof
```

(Ключ `-nodecount=10` ограничивает вывод десятью строками; без него таблица длиннее, итоги отличаются на 1 МБ из-за отсечения узлов.)

```
File: server
Type: alloc_space
Showing nodes accounting for -31959.15MB, 96.17% of 33233.01MB total
Dropped 218 nodes (cum <= 166.17MB)
Showing top 10 nodes out of 35
      flat  flat%   sum%        cum   cum%
-25259.19MB 76.01% 76.01% -30674.84MB 92.30%  compress/flate.NewWriter (inline)
-5278.55MB 15.88% 91.89% -5278.55MB 15.88%  compress/flate.(*compressor).initDeflate (inline)
 -914.49MB  2.75% 94.64%  -914.49MB  2.75%  compress/flate.(*dictDecoder).init (inline)
 -275.10MB  0.83% 95.47%  -275.10MB  0.83%  compress/flate.(*huffmanEncoder).generate
 -207.80MB  0.63% 96.09% -1122.29MB  3.38%  compress/flate.NewReader
  -22.52MB 0.068% 96.16% -1293.57MB  3.89%  compress/gzip.NewReader (inline)
   -3.50MB 0.011% 96.17% -32331.88MB 97.29%  main.main.Logging.func5.1
       1MB 0.003% 96.17% -23012.53MB 69.25%  github.com/magicelemental11/go-musthave-metrics-tpl/internal/handler.(*Handler).GetMetricJSON
    0.50MB 0.0015% 96.17% -7689.32MB 23.14%  text/template.(*Template).execute
    0.50MB 0.0015% 96.17% -32323.37MB 97.26%  github.com/magicelemental11/go-musthave-metrics-tpl/internal/middleware.Gzip.func1
```

### Бенчмарки до и после

Цифры ориентировочные: они зависят от `-benchtime` и прогрева, на коротких прогонах (например `-benchtime=200x`) значения заметно выше. Для сравнения запусков используйте `benchstat`.

| Бенчмарк | B/op до | B/op после | allocs/op до | allocs/op после |
|---|---|---|---|---|
| `ValueJSONGzipResponse` | 804 KiB | 8,5 KiB | 58 | 40 |
| `IndexPage` | 831 KiB | 18,5 KiB | 637 | 427 |
| `UpdatesBatchGzipSigned` | 61 KiB | 21 KiB | 165 | 160 |
| `UpdatesBatchGzip` | 59 KiB | 19 KiB | 139 | 134 |
| `ReportMetrics` (агент) | 1,72 MB | 32 KB | 323 | 248 |

Бенчмарки `UpdateMetricURL`, `UpdatesBatchPlain`, `MemStorage*`, `Sign`, `Valid` и `CollectMetrics` не затронуты изменениями: аллокации у них те же. `Sign`/`Valid` — около 1 мкс и 8–9 аллокаций, `MemStorageUpdateGauge`/`GetGauge`/`UpdateBatch` — без аллокаций.

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**
