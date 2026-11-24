-- Показ доп. задания «Индексы в БД»: один и тот же запрос с индексом и без него.
-- Выполнять по одной команде (Adminer http://localhost:8081 или psql), после go run ./cmd/seed.
-- Запросы — те же, что делает GET /api/tire-conditions?grip=1.15&page=1&limit=10:
-- первый считает total, второй выбирает страницу.

-- 0. Свежая статистика и карта видимости (нужна для Index Only Scan)
VACUUM ANALYZE tire_conditions;

-- 1. Индекс выключен
DROP INDEX IF EXISTS idx_tire_conditions_status_mu_tire;

EXPLAIN ANALYZE SELECT count(*) FROM tire_conditions WHERE status = 'published' AND mu_tire >= 1.15;
-- ожидается: Seq Scan on tire_conditions, Execution Time порядка 10–20 ms

EXPLAIN ANALYZE SELECT * FROM tire_conditions WHERE status = 'published' AND mu_tire >= 1.15 ORDER BY id LIMIT 10 OFFSET 4000;
-- ожидается: Seq Scan on tire_conditions, Execution Time порядка 15–20 ms

-- 2. Индекс включён
CREATE INDEX idx_tire_conditions_status_mu_tire ON tire_conditions (status, mu_tire);

EXPLAIN ANALYZE SELECT count(*) FROM tire_conditions WHERE status = 'published' AND mu_tire >= 1.15;
-- ожидается: Index Only Scan using idx_tire_conditions_status_mu_tire, Execution Time около 1 ms

EXPLAIN ANALYZE SELECT * FROM tire_conditions WHERE status = 'published' AND mu_tire >= 1.15 ORDER BY id LIMIT 10 OFFSET 4000;
-- ожидается: Bitmap Index Scan on idx_tire_conditions_status_mu_tire, Execution Time около 5 ms

-- Список индексов таблицы
SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'tire_conditions';
