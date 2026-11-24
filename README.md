# BrakePath — бэкенд расчёта тормозного пути

Веб-сервис на Go для приложения BrakePath. Пользователь публикует **услуги** — условия торможения (коэффициент сцепления шины `mu_tire`, коэффициент сцепления покрытия `mu_surface`, фото и 3D модель). Тормозной путь автомобиля считается по формуле `S = v² / (2 · mu_tire · mu_surface · 9.81)`.

Роли: **гость** (только чтение услуг) и **пользователь** (свои услуги, лайки). Пользователь с `is_moderator = true` может публиковать и удалять любые услуги.

## Стек

Go, gin, gorm, PostgreSQL, Minio (фото и 3D модели услуг), Redis (blacklist JWT, лента услуг), JWT, Swagger (swaggo).

## Запуск

```bash
docker compose up -d        # PostgreSQL :5432, Minio :9000/:9001, Redis :6379, Adminer :8081
go run ./cmd/tire-braking   # сервер на :8080
```

* Swagger: http://localhost:8080/swagger/index.html
* Таблицы `users`, `tire_conditions`, `likes` создаются вручную в Adminer (SQL из `migrations/`, по порядку номеров).
* Переменные окружения (необязательные): `JWT_SECRET`, `JWT_TTL` (по умолчанию `1h`), `REDIS_HOST`, `REDIS_PORT`, `ASYNC_SERVICE_URL` (по умолчанию `http://localhost:3001/api/rotation_async`).

## Таблицы базы данных

Каскадного удаления нет: услуги удаляются логически (статус `deleted`).

| Таблица | Назначение | Столбцы |
|---|---|---|
| `users` | пользователи | `id` PK, `username` VARCHAR(50), `password_hash` VARCHAR(255) (bcrypt), `is_moderator` BOOLEAN (по умолчанию false) |
| `tire_conditions` | услуги (условия торможения) | `id` PK, `name` VARCHAR(150), `description` TEXT, `description_en` VARCHAR(100) (краткое описание на английском, 50–100 символов), `status` VARCHAR(20) (`draft` / `published` / `deleted`), `image_url` VARCHAR(255) NOT NULL, `model_url` VARCHAR(255) NOT NULL (3D модель `.glb`), `mu_tire` NUMERIC(4,2), `mu_surface` NUMERIC(4,2), `angle_x` NUMERIC(5,2), `angle_y` NUMERIC(5,2) (углы поворота 3D модели в градусах, при создании услуги задаются случайно), `created_at` TIMESTAMP NOT NULL, `published_at` TIMESTAMP, `creator_id` FK → `users` |
| `likes` | лайки, связь M-M пользователь — услуга | `id` PK, `user_id` FK → `users`, `tire_condition_id` FK → `tire_conditions` |

`image_url` и `model_url` обязательные: если файл не выбран или недоступен, подставляются картинка и 3D модель по умолчанию (`/static/images/default_tire.jpg`, `/static/models/default_tire.glb`).

## Методы API

Всего 10 запросов в двух доменах. Все адреса начинаются с `/api`. Описание, примеры и коды ответов — в Swagger. Для методов с авторизацией нужен заголовок `Authorization: Bearer <access_token>`.

### Домен услуг: `/api/tire-conditions` (7 методов)

| Метод | Адрес | Доступ | Что делает |
|---|---|---|---|
| GET | `/api/tire-conditions` | все | страница списка опубликованных услуг: `?page=` (с 1), `?limit=` (по умолчанию 10, максимум 100), фильтр `?grip=` по минимальному `mu_tire`. Ответ: `total`, `page`, `limit`, `data` — массив услуг; `is_owner` — признак 0 или 1: создатель услуги совпадает с текущим пользователем (для гостя 0) |
| GET | `/api/tire-conditions/feed` | все | **одна** услуга ленты сессии (лента — 100 случайных услуг в Redis): без `id` — первая, `?id=N` — услуга N, `?id=N&next=true` — следующая в ленте после N, после 100-й формируются новые 100. В ответе `is_liked` — признак 0 или 1: стоит ли лайк текущего пользователя на услуге (для гостя 0), `position` — порядковый номер услуги в ленте |
| GET | `/api/tire-conditions/draft` | пользователь | черновик услуги текущего пользователя |
| POST | `/api/tire-conditions` | пользователь | добавление услуги (multipart: `name`, `description`, `mu_tire`, `mu_surface`, файлы `image` и `model` (3D модель `.glb`) уходят в Minio, в БД сохраняются их URL) |
| PUT | `/api/tire-conditions/{id}/publish` | владелец / модератор | публикация черновика (`draft` → `published`), поля `description`, `mu_tire`, `mu_surface` |
| DELETE | `/api/tire-conditions/{id}` | владелец / модератор | логическое удаление (SQL `UPDATE`, статус `deleted`) |
| POST | `/api/tire-conditions/{id}/like` | пользователь | поставить (`value: 1`) или снять (`value: 0`) лайк |

### Домен пользователей (3 метода)

| Метод | Адрес | Доступ | Что делает |
|---|---|---|---|
| POST | `/api/register` | все | регистрация, пароль хранится как bcrypt-хеш |
| POST | `/api/auth` | все | вход, ответ `access_token` (JWT, живёт 1 час) |
| POST | `/api/logout` | пользователь | выход: токен попадает в blacklist в Redis до истечения срока |

### Авторизация

1. `POST /api/auth` возвращает JWT (HS256, поля `user_id`, `is_moderator`, `iat`, `exp`).
2. Токен передаётся в заголовке `Authorization: Bearer <access_token>`.
3. Защищённые методы без токена, с просроченным или отозванным токеном отвечают `401`.
4. `POST /api/logout` кладёт токен в Redis: ключ `blacklist:<хеш токена>`, значение `user_id=<id>`, TTL равен оставшемуся сроку жизни токена.

Проверка в терминале:

```bash
docker exec -it <redis-контейнер> redis-cli
KEYS *
GET blacklist:<ключ из KEYS>
TTL blacklist:<ключ из KEYS>
```

## Доп. задание «Индексы в БД»

### 100 000 услуг

```bash
go run ./cmd/seed            # добавляет 100000 услуг (90 % опубликованных), создатели — seed_user_1..3
go run ./cmd/seed -n 250000  # другое количество
```

У сгенерированных услуг картинка — иллюстрация покрытия из `resources/images/surfaces` (`migrations/011_tire_conditions_surface_images.sql` для уже добавленных).

### Пагинация

`GET /api/tire-conditions?grip=1.15&page=2&limit=10` → `{"total": 4507, "page": 2, "limit": 10, "data": [...]}`. `limit` больше 100 уменьшается до 100.

### Индекс

Индекс `idx_tire_conditions_status_mu_tire` по полям `(status, mu_tire)` — по ним фильтрует список услуг. Создание: `migrations/007_tire_conditions_status_mu_tire_index.sql`. Показ разницы во времени запроса с индексом и без него (`EXPLAIN ANALYZE`): команды в `migrations/index_demo.sql`.

### Лента в Redis

Лента формируется при первом запросе `GET /api/tire-conditions/feed` от сессии: 100 случайных неповторяющихся опубликованных услуг. В Redis это список `feed:<сессия>` из id услуг; порядковый номер услуги = её место в списке (в ответе метода — поле `position`). Сессия гостя — кука `feed_session` (ключ `feed:guest:<значение куки>`), авторизованного пользователя — `feed:user:<id>`. Ключ живёт 1 час с последнего обращения. После 100-й карточки (`?id=<последняя>&next=true`) формируются новые 100.

```bash
docker exec -it brakepath_redis redis-cli
KEYS feed:*
LRANGE feed:guest:<значение куки> 0 -1   # 100 id услуг по порядку
LLEN feed:guest:<значение куки>          # 100
TTL feed:guest:<значение куки>
```

## Доп. задание «Межсервисное взаимодействие»

Второй сервис — `tire-braking-async` (NestJS, порт 3001), лежит в соседней папке. Сервисы общаются только по HTTP, к базе данных асинхронный сервис не обращается.

1. Запрос ленты с `next=true` (`GET /api/tire-conditions/feed?id=N&next=true`) — основной сервис отдаёт услугу и вызывает `POST http://localhost:3001/api/rotation_async` с её `id` и текущими углами.
2. Асинхронный сервис сразу отвечает `200`, через 5–10 секунд считает новые углы и вызывает метод основного сервиса.
3. `PUT /api/tire-conditions/{id}/angles`, тело `{"token": "bp3dTk01", "angle_x": 150.5, "angle_y": 75}` — запись новых углов. Вместо JWT — постоянный ключ `token` (8 байт): не совпал → `403`. Углы от 0 до 360, иначе `400`.
4. Новые углы видны повторным запросом ленты по `id` услуги: `GET /api/tire-conditions/feed?id=N` (поля `angle_x`, `angle_y`) — этим запросом фронтенд делает Short Polling.

Столбцы `angle_x`, `angle_y` добавляет `migrations/008_tire_conditions_angles.sql`. Вместо видео у услуги теперь 3D модель: столбец `video_url` переименован в `model_url` (`migrations/009_tire_conditions_model_url.sql`), в `POST /api/tire-conditions` файл передаётся в поле `model`.

## Доп. задание «Нативное приложение с ИИ»

Нативное приложение гостя (Tauri) лежит во фронтенде (`tire-braking-frontend/src-tauri`). Для него в веб-сервисе:

- **Поле `description_en`** — краткое описание услуги на английском (50–100 символов), `migrations/010_tire_conditions_description_en.sql`. Отдаётся в списке и ленте, задаётся при публикации (`PUT /api/tire-conditions/{id}/publish`, поле `description_en`: пустое или 50–100 символов латиницей, иначе `400`). По этому полю приложение считает текстовый эмбеддинг и ищет похожие карточки по загруженному изображению.
- **CORS** — middleware `CORS` (`internal/app/handler/middleware.go`): собранное приложение обращается к API напрямую по IP компьютера в локальной сети (`http://<IP>:8080`), а не через прокси фронтенда.

## HTML-страницы (лабораторные 1–2)

Страницы открываются в браузере без токена. Если токен передан, действия выполняются от его имени, без токена — от пользователя с `id = 1`.

| Адрес | Что показывает |
|---|---|
| `GET /feed/{id}` | лента: **одна** услуга (изображение, описание, лайки); `?next=true` — следующая услуга |
| `GET /tiles` | плитка услуг, фильтр `?grip=` |
| `GET /add` | добавление: «Далее» (название, фото и 3D модель, без выбора файлов подставляются картинка и модель по умолчанию) создаёт черновик, затем «Опубликовать» |
| `POST /add/create`, `POST /add/publish`, `POST /tiles/delete/{id}` | создание черновика, публикация, логическое удаление |

## Структура проекта

```
cmd/tire-braking          точка входа
internal/api              маршруты (server.go)
internal/app/handler      обработчики HTTP, middleware, DTO запросов
internal/app/repository   модели, DTO ответов, работа с БД (gorm) и Minio
internal/app/auth         JWT и blacklist
internal/app/config       настройки
internal/app/redis        подключение к Redis
migrations                SQL-миграции
templates, resources      HTML-шаблоны, стили и файлы по умолчанию
docs                      Swagger
```
