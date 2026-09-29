-- Заявки (запросы на расчёт тормозного пути) и их состав

CREATE TABLE requests (
    id SERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'formed', 'completed', 'rejected', 'deleted')),
    creator_id INTEGER NOT NULL REFERENCES users(id),
    moderator_id INTEGER REFERENCES users(id),
    speed_kmh INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    formed_at TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE TABLE request_items (
    id SERIAL PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES requests(id),
    tire_condition_id INTEGER NOT NULL REFERENCES tire_conditions(id),
    braking_distance NUMERIC(6, 2)
);
