// Генератор тестовых данных для доп. задания «Индексы в БД»: добавляет в tire_conditions много опубликованных услуг.
//
// Запуск из корня репозитория (PostgreSQL из docker-compose должен быть запущен):
//
//	go run ./cmd/seed            // 100000 услуг
//	go run ./cmd/seed -n 250000  // другое количество
//
// Создателями услуг становятся служебные пользователи seed_user_1..3 (войти под ними нельзя — пароль случайный).
// Удалить сгенерированные услуги:
//
//	DELETE FROM tire_conditions WHERE creator_id IN (SELECT id FROM users WHERE username LIKE 'seed_user_%');
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	mrand "math/rand"
	"time"

	"tire-braking/internal/app/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// тот же адрес БД, что и в repository.NewRepository
const dsn = "host=localhost user=postgres password=postgres dbname=brakepath port=5432 sslmode=disable"

const (
	seedUsers = 3
	batchSize = 1000
)

type surface struct {
	ru, en string
	mu     float64 // типичный коэффициент сцепления покрытия
	image  string  // иллюстрация покрытия из resources/images/surfaces
}

var (
	brands = []string{
		"Michelin Pilot Sport", "Bridgestone Blizzak", "Continental PremiumContact", "Pirelli P Zero",
		"Nokian Hakkapeliitta", "Goodyear Eagle F1", "Yokohama BluEarth", "Dunlop SP Sport",
		"Hankook Ventus", "Toyo Proxes",
	}
	kinds = []string{"Summer", "Winter", "All-season", "Sport", "Touring"}
	roads = []surface{
		{"сухой асфальт", "dry asphalt", 0.80, "/static/images/surfaces/dry_asphalt.jpg"},
		{"мокрый асфальт", "wet asphalt", 0.50, "/static/images/surfaces/wet_asphalt.jpg"},
		{"бетон", "concrete", 0.75, "/static/images/surfaces/concrete.jpg"},
		{"гравий", "gravel", 0.45, "/static/images/surfaces/gravel.jpg"},
		{"укатанный снег", "packed snow", 0.25, "/static/images/surfaces/packed_snow.jpg"},
		{"лёд", "ice", 0.12, "/static/images/surfaces/ice.jpg"},
	}
)

func main() {
	n := flag.Int("n", 100000, "сколько услуг добавить")
	flag.Parse()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("не удалось подключиться к БД: %v", err)
	}

	creators := make([]uint, 0, seedUsers)
	for i := 1; i <= seedUsers; i++ {
		creators = append(creators, ensureSeedUser(db, fmt.Sprintf("seed_user_%d", i)))
	}

	rnd := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	now := time.Now()

	batch := make([]repository.TireCondition, 0, batchSize)
	for i := 1; i <= *n; i++ {
		batch = append(batch, randomTireCondition(rnd, now, creators))
		if len(batch) == batchSize || i == *n {
			if err := db.CreateInBatches(batch, batchSize).Error; err != nil {
				log.Fatalf("не удалось добавить услуги: %v", err)
			}
			batch = batch[:0]
			if i%(10*batchSize) == 0 || i == *n {
				log.Printf("добавлено %d из %d", i, *n)
			}
		}
	}

	// свежая статистика для планировщика запросов, чтобы EXPLAIN ANALYZE показывал реальный план
	if err := db.Exec("ANALYZE tire_conditions").Error; err != nil {
		log.Printf("ANALYZE не выполнен: %v", err)
	}

	var total int64
	db.Model(&repository.TireCondition{}).Count(&total)
	log.Printf("готово, всего услуг в таблице: %d", total)
}

// ensureSeedUser — id служебного пользователя; если его нет, создаётся с неизвестным случайным паролем
func ensureSeedUser(db *gorm.DB, username string) uint {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		log.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(secret)), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	user := repository.User{Username: username, PasswordHash: string(hash)}
	if err := db.Where("username = ?", username).FirstOrCreate(&user).Error; err != nil {
		log.Fatalf("не удалось получить пользователя %s: %v", username, err)
	}
	return user.ID
}

func randomTireCondition(rnd *mrand.Rand, now time.Time, creators []uint) repository.TireCondition {
	road := roads[rnd.Intn(len(roads))]
	brand := brands[rnd.Intn(len(brands))]
	kind := kinds[rnd.Intn(len(kinds))]

	// mu_tire 0.10–1.20 и mu_surface около типичного значения покрытия, NUMERIC(4,2): два знака после запятой
	muTire := round2(0.10 + rnd.Float64()*1.10)
	muSurface := round2(road.mu * (0.85 + rnd.Float64()*0.30))
	if muSurface < 0.05 {
		muSurface = 0.05
	}

	created := now.Add(-time.Duration(rnd.Int63n(int64(730 * 24 * time.Hour))))

	tc := repository.TireCondition{
		Name:          fmt.Sprintf("%s, %s", brand, road.ru),
		Description:   fmt.Sprintf("Условия торможения: %s, %s. Сцепление шины %.2f, покрытия %.2f.", brand, road.ru, muTire, muSurface),
		DescriptionEn: describe(kind, road.en, muTire, muSurface),
		Status:        randomStatus(rnd),
		ImageURL:      road.image,
		ModelURL:      repository.DefaultModelURL,
		MuTire:        muTire,
		MuSurface:     muSurface,
		AngleX:        repository.RandomAngle(),
		AngleY:        repository.RandomAngle(),
		CreatedAt:     created,
		CreatorID:     creators[rnd.Intn(len(creators))],
	}
	if tc.Status == "published" {
		published := created.Add(time.Duration(rnd.Int63n(int64(7 * 24 * time.Hour))))
		if published.After(now) {
			published = now
		}
		tc.PublishedAt = &published
	}
	return tc
}

// randomStatus — 90 % опубликованных, по 5 % черновиков и удалённых
func randomStatus(rnd *mrand.Rand) string {
	switch p := rnd.Intn(100); {
	case p < 90:
		return "published"
	case p < 95:
		return "draft"
	default:
		return "deleted"
	}
}

// describe — краткое описание на английском (50–100 символов), как требуется для поиска по эмбеддингам CLIP
func describe(kind, road string, muTire, muSurface float64) string {
	grip := "high"
	switch {
	case muTire < 0.4:
		grip = "low"
	case muTire < 0.8:
		grip = "medium"
	}

	distance := "short"
	switch k := muTire * muSurface; {
	case k < 0.25:
		distance = "long"
	case k < 0.5:
		distance = "medium"
	}

	return fmt.Sprintf("%s tires on %s, %s grip and %s braking distance", kind, road, grip, distance)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
