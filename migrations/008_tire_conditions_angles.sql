-- Доп. задание «Межсервисное взаимодействие»: два угла поворота 3D модели услуги, в градусах (0–360).
-- У новых услуг углы задаются случайно в коде, у уже существующих — случайно здесь.

ALTER TABLE tire_conditions ADD COLUMN IF NOT EXISTS angle_x NUMERIC(5,2) NOT NULL DEFAULT 0;
ALTER TABLE tire_conditions ADD COLUMN IF NOT EXISTS angle_y NUMERIC(5,2) NOT NULL DEFAULT 0;

UPDATE tire_conditions
SET angle_x = round((random() * 359.99)::numeric, 2),
    angle_y = round((random() * 359.99)::numeric, 2);
