package feed

import (
	"context"
	"errors"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	// Size — сколько случайных неповторяющихся услуг входит в ленту сессии
	Size = 100
	// TTL — время жизни ленты в Redis, продлевается при каждом обращении
	TTL = time.Hour

	keyPrefix = "feed:"
)

// ErrEmpty — в БД нет опубликованных услуг, ленту сформировать не из чего
var ErrEmpty = errors.New("no published tire conditions")

// IDSource — откуда берутся id случайных опубликованных услуг (репозиторий)
type IDSource interface {
	RandomPublishedIDs(limit int) ([]uint, error)
}

// Feed — лента услуг в Redis.
// Ключ feed:<сессия> — список (list) из Size id услуг; порядковый номер услуги в ленте = индекс в списке + 1.
// Сессия: guest:<id из куки feed_session> для гостя и user:<id> для авторизованного пользователя.
type Feed struct {
	client *goredis.Client
	src    IDSource
}

func New(client *goredis.Client, src IDSource) *Feed {
	return &Feed{client: client, src: src}
}

func key(session string) string { return keyPrefix + session }

// Ensure — лента сессии в порядке показа; если её нет в Redis (первая загрузка или истёк TTL), формируется новая
func (f *Feed) Ensure(ctx context.Context, session string) ([]uint, error) {
	ids, err := f.load(ctx, session)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}
	return f.Rebuild(ctx, session)
}

// Rebuild — формирует новую ленту сессии: Size случайных неповторяющихся опубликованных услуг;
// старая лента заменяется (вызывается, когда просмотрены все карточки предыдущей)
func (f *Feed) Rebuild(ctx context.Context, session string) ([]uint, error) {
	ids, err := f.src.RandomPublishedIDs(Size)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, ErrEmpty
	}

	values := make([]interface{}, len(ids))
	for i, id := range ids {
		values[i] = strconv.FormatUint(uint64(id), 10)
	}

	k := key(session)
	pipe := f.client.TxPipeline()
	pipe.Del(ctx, k)
	pipe.RPush(ctx, k, values...)
	pipe.Expire(ctx, k, TTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// Position — порядковый номер id в ленте (с 1) и индекс (с 0); если услуги нет в ленте — 0 и -1
func Position(ids []uint, id uint) (position, index int) {
	for i, v := range ids {
		if v == id {
			return i + 1, i
		}
	}
	return 0, -1
}

func (f *Feed) load(ctx context.Context, session string) ([]uint, error) {
	k := key(session)
	values, err := f.client.LRange(ctx, k, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]uint, 0, len(values))
	for _, v := range values {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, uint(n))
	}
	if err := f.client.Expire(ctx, k, TTL).Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
