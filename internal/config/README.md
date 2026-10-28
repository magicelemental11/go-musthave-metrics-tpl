# internal/config

Конфигурация сервера и агента. Источники по убыванию приоритета: переменная окружения, флаг, JSON-файл, значение по умолчанию.

Файл задаётся флагом `-c` / `-config` или переменной `CONFIG`. Неизвестные поля в файле считаются ошибкой. Интервалы в файле пишутся строкой (`"1s"`, `"500ms"`), во флагах и переменных окружения остаются целыми секундами.

## Сервер

| Опция | Флаг | Переменная | Ключ в JSON | По умолчанию |
|---|---|---|---|---|
| Адрес | `-a` | `ADDRESS` | `address` | `localhost:8080` |
| Интервал сохранения | `-i` | `STORE_INTERVAL` | `store_interval` | `300s` (0 — синхронно) |
| Файл хранилища | `-f` | `FILE_STORAGE_PATH` | `store_file` | `metrics-db.json` (пусто — выключено) |
| Восстановление | `-r` | `RESTORE` | `restore` | `true` |
| DSN базы | `-d` | `DATABASE_DSN` | `database_dsn` | пусто |
| Ключ подписи | `-k` | `KEY` | `key` | пусто |
| Приватный ключ RSA | `-crypto-key` | `CRYPTO_KEY` | `crypto_key` | пусто |
| Файл аудита | `-audit-file` | `AUDIT_FILE` | `audit_file` | пусто |
| URL аудита | `-audit-url` | `AUDIT_URL` | `audit_url` | пусто |
| Адрес pprof | `-pprof-address` | `PPROF_ADDRESS` | `pprof_address` | пусто |
| Доверенная подсеть агентов (CIDR) | `-t` | `TRUSTED_SUBNET` | `trusted_subnet` | пусто (проверки нет) |

```json
{
    "address": "localhost:8080",
    "restore": true,
    "store_interval": "1s",
    "store_file": "/path/to/file.db",
    "database_dsn": "",
    "crypto_key": "/path/to/private.pem",
    "trusted_subnet": "192.168.1.0/24"
}
```

## Агент

| Опция | Флаг | Переменная | Ключ в JSON | По умолчанию |
|---|---|---|---|---|
| Адрес сервера | `-a` | `ADDRESS` | `address` | `localhost:8080` |
| Интервал опроса | `-p` | `POLL_INTERVAL` | `poll_interval` | `2s` (минимум 1s) |
| Интервал отправки | `-r` | `REPORT_INTERVAL` | `report_interval` | `10s` (минимум 1s) |
| Ключ подписи | `-k` | `KEY` | `key` | пусто |
| Публичный ключ RSA | `-crypto-key` | `CRYPTO_KEY` | `crypto_key` | пусто |
| Лимит запросов | `-l` | `RATE_LIMIT` | `rate_limit` | `1` |

```json
{
    "address": "localhost:8080",
    "report_interval": "1s",
    "poll_interval": "1s",
    "crypto_key": "/path/to/public.pem"
}
```
