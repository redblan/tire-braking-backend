package repository

import "fmt"

type Repository struct{}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type Service struct {
	ID              int
	Name            string
	Description     string
	Status          string
	ImageURL        string
	VideoURL        string
	TireType        string
	RoadCondition   string
	GripCoefficient float64
	Likes           int
}

func (r *Repository) GetServices() ([]Service, error) {
	services := []Service{
		{ID: 1, Name: "Летняя шина, сухой асфальт", Status: "published",
			Description: "Оптимальные условия торможения. Летняя шина на сухом асфальте обеспечивает максимальное сцепление.",
			VideoURL:    "http://localhost:9000/brakes/braking_technique_demo.mp4",
			ImageURL:    "http://localhost:9000/brakes/tire_summer.jpg",
			TireType:    "летняя", RoadCondition: "сухой асфальт", GripCoefficient: 0.75, Likes: 12},
		{ID: 2, Name: "Зимняя шина, гололёд", Status: "published",
			Description: "Экстренное торможение на зимней резине по гололёду. Сцепление значительно снижено.",
			VideoURL:    "http://localhost:9000/brakes/emergency_braking_demo.mp4",
			ImageURL:    "http://localhost:9000/brakes/tire_winter.jpg",
			TireType:    "зимняя", RoadCondition: "гололёд", GripCoefficient: 0.15, Likes: 7},
		{ID: 3, Name: "Черновик расчёта", Status: "draft",
			Description: "Черновик, ещё не опубликован.",
			VideoURL:    "http://localhost:9000/brakes/braking_technique_demo.mp4",
			ImageURL:    "http://localhost:9000/brakes/tire_summer.jpg",
			TireType:    "всесезонная", RoadCondition: "мокрый асфальт", GripCoefficient: 0.45},
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

func (r *Repository) GetServicesByMinGrip(minGrip float64) ([]Service, error) {
	services, _ := r.GetServices()
	var result []Service
	for _, s := range services {
		if s.Status == "published" && s.GripCoefficient >= minGrip {
			result = append(result, s)
		}
	}
	return result, nil
}
