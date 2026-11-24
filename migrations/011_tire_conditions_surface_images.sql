-- Картинки для сгенерированных услуг (cmd/seed): вместо общей картинки по умолчанию — иллюстрация покрытия.
-- Покрытие берётся из названия услуги («<шина>, <покрытие>»). Услуги, добавленные пользователями, не меняются.

UPDATE tire_conditions
SET image_url = CASE
        WHEN name LIKE '%, сухой асфальт' THEN '/static/images/surfaces/dry_asphalt.jpg'
        WHEN name LIKE '%, мокрый асфальт' THEN '/static/images/surfaces/wet_asphalt.jpg'
        WHEN name LIKE '%, бетон' THEN '/static/images/surfaces/concrete.jpg'
        WHEN name LIKE '%, гравий' THEN '/static/images/surfaces/gravel.jpg'
        WHEN name LIKE '%, укатанный снег' THEN '/static/images/surfaces/packed_snow.jpg'
        WHEN name LIKE '%, лёд' THEN '/static/images/surfaces/ice.jpg'
        ELSE image_url
    END
WHERE image_url = '/static/images/default_tire.jpg'
  AND creator_id IN (SELECT id FROM users WHERE username LIKE 'seed_user_%');
