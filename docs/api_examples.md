# API Examples

**Base URL:** `http://localhost:8080`

## 1. Health Check

Проверка работоспособности сервиса.

```bash
curl -X GET http://localhost:8080/health
```

**Ответ:**
```
OK
```

---

## 2. Создать оффер (POST /offers)

Создает оффер на аренду самоката для пользователя.

```bash
curl -X POST http://localhost:8080/offers \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-1",
    "scooter_id": "scooter-1"
  }'
```

**Пример ответа (201 Created):**
```json
{
    "created_at": "2025-11-30T08:04:42.286687216Z",
    "deposit": 0,
    "expires_at": "2025-11-30T08:14:42.286687216Z",
    "id": "e7094cc1-3987-4f20-af2a-4ed9f6dda7c9",
    "price_per_minute": 8,
    "price_unlock": 0,
    "scooter_id": "scooter-1",
    "user_id": "user-1",
    "zone_id": "zone-1"
}
```

**Возможные ошибки:**
- `400 Bad Request` - некорректные параметры
- `503 Service Unavailable` - внешний сервис недоступен

---

## 3. Создать заказ (POST /orders)

Создает заказ на основе оффера. Требует `order_id` от клиента для идемпотентности.

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "order-abc-123",
    "offer_id": "offer-789",
    "user_id": "user-123"
  }'
```

**Пример ответа (201 Created):**
```json
{
    "current_amount": 0,
    "deposit": 0,
    "duration_seconds": null,
    "finish_time": null,
    "id": "order-test",
    "offer_id": "e7094cc1-3987-4f20-af2a-4ed9f6dda7c9",
    "price_per_minute": 8,
    "price_unlock": 0,
    "scooter_id": "scooter-1",
    "start_time": "2025-11-30T08:06:12.740439701Z",
    "status": "ACTIVE",
    "user_id": "user-1"
}
```

**Возможные ошибки:**
- `400 Bad Request` - оффер невалиден, истек или уже использован
- `500 Internal Server Error` - внутренняя ошибка сервера

---

## 4. Получить информацию о заказе (GET /orders/{order_id})

Возвращает информацию о заказе по его ID.

```bash
curl -X GET http://localhost:8080/orders/order-abc-123
```

**Пример ответа (200 OK):**
```json
{
    "current_amount": 0,
    "deposit": 0,
    "duration_seconds": null,
    "finish_time": null,
    "id": "order-test",
    "offer_id": "e7094cc1-3987-4f20-af2a-4ed9f6dda7c9",
    "price_per_minute": 8,
    "price_unlock": 0,
    "scooter_id": "scooter-1",
    "start_time": "2025-11-30T08:06:12.740439701Z",
    "status": "ACTIVE",
    "user_id": "user-1"
}
```

**Для завершенного заказа:**
```json
{
    "current_amount": 16,
    "deposit": 0,
    "duration_seconds": 73,
    "finish_time": "2025-11-30T08:07:26.699161Z",
    "id": "order-test",
    "offer_id": "e7094cc1-3987-4f20-af2a-4ed9f6dda7c9",
    "price_per_minute": 8,
    "price_unlock": 0,
    "scooter_id": "scooter-1",
    "start_time": "2025-11-30T08:06:12.740439Z",
    "status": "FINISHED",
    "user_id": "user-1"
}
```

**Возможные ошибки:**
- `404 Not Found` - заказ не найден
- `500 Internal Server Error` - внутренняя ошибка сервера

---

## 5. Завершить заказ (POST /orders/{order_id}/finish)

Завершает активный заказ, списывает стоимость поездки и размораживает депозит.

```bash
curl -X POST http://localhost:8080/orders/order-abc-123/finish
```

**Пример ответа (200 OK):**
```json
{
    "current_amount": 16,
    "deposit": 0,
    "duration_seconds": 73,
    "finish_time": "2025-11-30T08:07:26.699161Z",
    "id": "order-test",
    "offer_id": "e7094cc1-3987-4f20-af2a-4ed9f6dda7c9",
    "price_per_minute": 8,
    "price_unlock": 0,
    "scooter_id": "scooter-1",
    "start_time": "2025-11-30T08:06:12.740439Z",
    "status": "FINISHED",
    "user_id": "user-1"
}
```

**Возможные ошибки:**
- `400 Bad Request` - заказ не найден
- `409 Conflict` - заказ уже завершен (идемпотентность)
- `500 Internal Server Error` - ошибка завершения

---

## Полный пример workflow

### Шаг 1: Создать оффер
```bash
OFFER_RESPONSE=$(curl -s -X POST http://localhost:8080/offers \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-1",
    "scooter_id": "scooter-1"
  }')

OFFER_ID=$(echo $OFFER_RESPONSE | jq -r '.id')
echo "Created offer: $OFFER_ID"
```

### Шаг 2: Создать заказ
```bash
ORDER_ID="order-$(uuidgen)"
ORDER_RESPONSE=$(curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d "{
    \"order_id\": \"$ORDER_ID\",
    \"offer_id\": \"$OFFER_ID\",
    \"user_id\": \"user-1\"
  }")

echo "Created order: $ORDER_ID"
```

### Шаг 3: Проверить статус заказа
```bash
curl -s -X GET http://localhost:8080/orders/$ORDER_ID | jq
```

### Шаг 4: Завершить заказ
```bash
curl -s -X POST http://localhost:8080/orders/$ORDER_ID/finish | jq
```
