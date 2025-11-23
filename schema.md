client/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── offers/
│   │   │   ├── model.go          # Offer struct
│   │   │   ├── repository.go     # OfferRepository interface
│   │   │   └── service.go        # Бизнес-логика создания офферов; in-memory кэши: zones (TTL 10m), configs (∞ TTL)
│   │   ├── orders/
│   │   │   ├── model.go          # Order struct
│   │   │   ├── repository.go     # OrderRepository interface
│   │   │   └── service.go        # Бизнес-логика заказов
│   │   └── pricing/
│   │       └── calculator.go     # Расчет цен (surge, скидки)
│   │
│   ├── storage/
│   │   ├── postgres/
│   │   │   ├── postgres.go       # Подключение к DB
│   │   │   └── order_repo.go     # Реализация OrderRepository (заказы и транзакции)
│   │   └── redis/
│   │       ├── redis.go          # Подключение к Redis
│   │       ├── offer_repo.go     # Реализация OfferRepository (офферы - основное хранилище)
│   │       └── cache.go          # Кэш для заказов (зоны кэшируются в памяти сервиса)
│   │
│   ├── external/
│   │   ├── client.go             # HTTP клиент к external API
│   │   └── models.go             # DTOs для external API
│   │
│   ├── handler/
│   │   ├── handler.go            # Базовые HTTP handlers
│   │   ├── offers.go             # Handlers для офферов
│   │   └── orders.go             # Handlers для заказов (POST /orders ожидает client-provided order_id)
│   │
│   └── config/
│       └── config.go
│
├── migrations/
│   └── 001_create_orders_and_paynents.sql    # Создание таблиц orders и payment_transactions
├── api/
│   └── api.gen.go
└── go.mod
