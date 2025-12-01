client/
├── cmd/
│   └── server/
│       └── main.go                            # Точка входа: инициализация зависимостей, запуск HTTP сервера
│
├── internal/
│   ├── domain/
│   │   ├── offers/
│   │   │   ├── repository.go                  # OfferRepository interface
│   │   │   └── service.go                     # Бизнес-логика создания офферов; in-memory кэши: zones (TTL 10m), configs (∞ TTL)
│   │   ├── orders/
│   │   │   ├── repository.go                  # OrderRepository interface
│   │   │   └── service.go                     # Бизнес-логика заказов (создание, получение, завершение)
│   │   └── pricing/
│   │       └── calculator.go                  # Расчет цен (surge, скидки при низком заряде, подписки)
│   │
│   ├── storage/
│   │   ├── postgres/
│   │   │   ├── postgres.go                    # Подключение к PostgreSQL
│   │   │   └── order_repo.go                  # Реализация OrderRepository (заказы и payment_transactions)
│   │   └── redis/
│   │       ├── redis.go                       # Подключение к Redis
│   │       ├── offer_repo.go                  # Реализация OfferRepository (основное хранилище офферов)
│   │       └── cache.go                       # OrderCache для кэширования заказов
│   │
│   ├── external/
│   │   ├── client.go                          # HTTP клиент к external API
│   │   ├── models.go                          # DTOs для external API (ScooterData, TariffZone, UserProfile, DynamicConfigs)
│   │   └── api_client.gen.go                  # Сгенерированный клиент для external API
│   │
│   ├── handler/
│   │   ├── offers.go                          # HTTP handlers для офферов (POST /offers)
│   │   └── orders.go                          # HTTP handlers для заказов (POST /orders, GET /orders/{id}, POST /orders/{id}/finish)
│   │
│   ├── helpers/
│   │   └── request_logger.go                  # Middleware для структурированного логирования запросов (zerolog)
│   │
│   ├── jobs/
│   │   └── order_cleanup.go                   # Фоновая джоба для очистки старых заказов
│   │
│   └── config/
│       └── config.go                          # Загрузка конфигурации из переменных окружения
│
├── migrations/
│   └── 001_create_orders_and_payments.sql    # Создание таблиц orders и payment_transactions
│
├── api/
│   └── api.gen.go                            # Сгенерированные типы и роутеры из OpenAPI спецификации
│
└── go.mod                                    # Go модуль с зависимостями
