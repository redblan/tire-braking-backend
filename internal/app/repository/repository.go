package repository

import "fmt"

type Repository struct{}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type Service struct {
	ID          int
	Name        string
	Description string
	Status      string
	ImageURL    string
	VideoURL    string
	TireGrip    float64 // коэффициент сцепления шины
	SurfaceGrip float64 // коэффициент сцепления покрытия
	Distance    float64 // тормозной путь при 60 км/ч, м
	Likes       int
}

func (r *Repository) GetServices() ([]Service, error) {
	services := []Service{
		{ID: 1, Name: "Летняя шина, сухой асфальт", Status: "published",
			Description: "Оптимальные условия торможения. Летняя шина на сухом асфальте.",
			VideoURL:    "http://localhost:9000/brakes/braking_technique_demo.mp4",
			ImageURL:    "http://localhost:9000/brakes/tire_summer.jpg",
			TireGrip:    0.90, SurfaceGrip: 0.85, Distance: 18.5, Likes: 12},
		{ID: 2, Name: "Зимняя шина, гололёд", Status: "published",
			Description: "Экстренное торможение на зимней резине по гололёду.",
			VideoURL:    "http://localhost:9000/brakes/emergency_braking_demo.mp4",
			ImageURL:    "http://localhost:9000/brakes/tire_winter.jpg",
			TireGrip:    0.80, SurfaceGrip: 0.20, Distance: 88.5, Likes: 7},
		{ID: 3, Name: "Черновик расчёта", Status: "draft",
			Description: "Черновик, ещё не опубликован.",
			VideoURL:    "http://localhost:9000/brakes/braking_technique_demo.mp4",
			ImageURL:    "http://localhost:9000/brakes/tire_summer.jpg",
			TireGrip:    0.80, SurfaceGrip: 0.50, Distance: 35.4},
	}
	return services, nil
}

func (r *Repository) GetService(id int, next bool) (Service, error) {
	services, _ := r.GetServices()
	for i, s := range services {
		if s.ID == id {
			if next && i+1 < len(services) {
				return services[i+1], nil
			}
			return s, nil
		}
	}
	return Service{}, fmt.Errorf("услуга не найдена")
}

func (r *Repository) GetDraft() (Service, error) {
	services, _ := r.GetServices()
	for _, s := range services {
		if s.Status == "draft" {
			return s, nil
		}
	}
	return Service{}, fmt.Errorf("черновик не найден")
}

func (r *Repository) GetServicesByMinTireGrip(minGrip float64) ([]Service, error) {
	services, _ := r.GetServices()
	var result []Service
	for _, s := range services {
		if s.Status == "published" && s.TireGrip >= minGrip {
			result = append(result, s)
		}
	}
	return result, nil
}
