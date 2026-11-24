-- Доп. задание «Индексы в БД».
-- Список услуг (GET /api/tire-conditions) фильтрует по статусу и минимальному mu_tire:
--   WHERE status = 'published' AND mu_tire >= ?
-- Составной индекс по этим двум полям ускоряет и подсчёт total, и выборку страницы.

CREATE INDEX IF NOT EXISTS idx_tire_conditions_status_mu_tire ON tire_conditions (status, mu_tire);
