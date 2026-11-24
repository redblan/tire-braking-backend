-- Доп. задание «Нативное приложение с ИИ»: новое поле услуги — краткое описание на английском (50–100 символов).
-- По нему в приложении считается текстовый эмбеддинг для поиска похожих карточек по изображению.

ALTER TABLE tire_conditions ADD COLUMN IF NOT EXISTS description_en VARCHAR(100) NOT NULL DEFAULT '';

-- У сгенерированных услуг (cmd/seed) английское описание лежало в description: переносим его в новое поле,
-- а в description ставим описание на русском
UPDATE tire_conditions
SET description_en = description,
    description = 'Условия торможения: ' || name || '. Сцепление шины ' || mu_tire || ', покрытия ' || mu_surface || '.'
WHERE description_en = '' AND description ~ '^[ -~]{50,100}$';

-- Остальным услугам английское описание собирается из названия и коэффициентов
UPDATE tire_conditions
SET description_en =
    (CASE WHEN name ILIKE '%зимн%' THEN 'Winter'
          WHEN name ILIKE '%летн%' THEN 'Summer'
          WHEN name ILIKE '%спорт%' THEN 'Sport'
          ELSE 'All-season' END)
    || ' tires on ' ||
    (CASE WHEN name ILIKE '%лёд%' OR name ILIKE '%лед%' OR name ILIKE '%льд%' THEN 'ice'
          WHEN name ILIKE '%снег%' OR name ILIKE '%снеж%' THEN 'packed snow'
          WHEN name ILIKE '%мокр%' OR name ILIKE '%дожд%' THEN 'wet asphalt'
          WHEN name ILIKE '%грав%' OR name ILIKE '%щеб%' THEN 'gravel'
          WHEN name ILIKE '%бетон%' THEN 'concrete'
          WHEN name ILIKE '%грунт%' OR name ILIKE '%гряз%' THEN 'dirt road'
          ELSE 'dry asphalt' END)
    || ', ' ||
    (CASE WHEN COALESCE(mu_tire, 0) < 0.4 THEN 'low'
          WHEN mu_tire < 0.8 THEN 'medium'
          ELSE 'high' END)
    || ' grip and ' ||
    (CASE WHEN COALESCE(mu_tire * mu_surface, 0) < 0.25 THEN 'long'
          WHEN mu_tire * mu_surface < 0.5 THEN 'medium'
          ELSE 'short' END)
    || ' braking distance'
WHERE description_en = '';
