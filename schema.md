client/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── offers/
│   │   │   ├── model.go          # Offer struct
│   │   │   ├── repository.go     # OfferRepository interface
│   │   │   └── service.go        # Бизнес-логика создания офферов
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
│   │       └── cache.go          # Кэш для заказов и зон
│   │
│   ├── external/
│   │   ├── client.go             # HTTP клиент к external API
│   │   └── models.go             # DTOs для external API
│   │
│   ├── handler/
│   │   ├── handler.go            # Базовые HTTP handlers
│   │   ├── offers.go             # Handlers для офферов
│   │   └── orders.go             # Handlers для заказов
│   │
│   └── config/
│       └── config.go
│
├── migrations/
│   └── 001_create_orders.sql    # Создание таблиц orders и payment_transactions
├── api/
│   └── api.gen.go
└── go.mod

