-- URL изображения и видео услуги обязательны: код всегда заполняет оба поля
-- (файлы по умолчанию /static/images/default_tire.jpg и /static/videos/default_tire.mp4)

UPDATE tire_conditions SET image_url = '/static/images/default_tire.jpg' WHERE image_url IS NULL OR image_url = '';
UPDATE tire_conditions SET video_url = '/static/videos/default_tire.mp4' WHERE video_url IS NULL OR video_url = '';

ALTER TABLE tire_conditions ALTER COLUMN image_url SET NOT NULL;
ALTER TABLE tire_conditions ALTER COLUMN video_url SET NOT NULL;
