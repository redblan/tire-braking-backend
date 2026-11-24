-- Доп. задание «Межсервисное взаимодействие»: у услуги вместо видео — 3D модель (.glb).
-- Столбец video_url переименовывается в model_url. Старые ссылки на видео моделью не являются,
-- поэтому у таких строк ставится модель по умолчанию (сами видеофайлы в Minio не удаляются).

ALTER TABLE tire_conditions RENAME COLUMN video_url TO model_url;

UPDATE tire_conditions
SET model_url = '/static/models/default_tire.glb'
WHERE model_url NOT LIKE '%.glb';
