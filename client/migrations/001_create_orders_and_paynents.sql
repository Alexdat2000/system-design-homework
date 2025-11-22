CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    scooter_id VARCHAR(255) NOT NULL,
    offer_id VARCHAR(255) NOT NULL,
    price_per_minute INTEGER NOT NULL,
    price_unlock INTEGER NOT NULL,
    deposit INTEGER NOT NULL,
    total_amount INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('ACTIVE', 'FINISHED', 'CANCELLED', 'PAYMENT_FAILED')),
    start_time TIMESTAMP NOT NULL,
    finish_time TIMESTAMP,
    duration_seconds INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS payment_transactions (
    id VARCHAR(255) PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    transaction_type VARCHAR(50) NOT NULL CHECK (transaction_type IN ('HOLD', 'CLEAR', 'REFUND')),
    amount INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('SUCCESS', 'FAILED', 'PENDING')),
    external_transaction_id VARCHAR(255),
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_payment_transactions_order_id FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);