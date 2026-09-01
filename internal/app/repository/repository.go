package repository

import (
	"fmt"
)

type Repository struct{}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type Service struct {
	ID              int
	Name            string // например "Зимняя шина на гололёде"
	Description     string
	Status          string // draft / published / deleted
	ImageURL        string
	VideoURL        string
	TireType        string  // тип_резины
	RoadCondition   string  // дорожное_условие
	GripCoefficient float64 // коэффициент_сцепления — числовое поле для фильтра
	Likes           int
}

func (r *Repository) GetServices() ([]Service, error) {
	services := []Service{
		{ID: 1, Name: "Летняя шина, сухой асфальт", Status: "published", TireType: "летняя", RoadCondition: "сухой асфальт", GripCoefficient: 0.8, Likes: 12},
		{ID: 2, Name: "Зимняя шина, гололёд", Status: "published", TireType: "зимняя", RoadCondition: "гололёд", GripCoefficient: 0.15, Likes: 7},
		{ID: 3, Name: "Черновик расчёта", Status: "draft", TireType: "всесезонная", RoadCondition: "мокрый асфальт", GripCoefficient: 0.4},
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("коллекция пуста")
	}
	return services, nil
}

// GetService: если next=true — вернуть следующую услугу после ID (для кнопки "следующий")
func (r *Repository) GetService(id int, next bool) (Service, error) {
	services, err := r.GetServices()
	if err != nil {
		return Service{}, err
	}
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
	services, err := r.GetServices()
	if err != nil {
		return Service{}, err
	}
	for _, s := range services {
		if s.Status == "draft" {
			return s, nil
		}
	}
	return Service{}, fmt.Errorf("черновик не найден")
}

// фильтр по теме — числовое поле, не текстовый Contains как в примере методички
func (r *Repository) GetServicesByMinGrip(minGrip float64) ([]Service, error) {
	services, err := r.GetServices()
	if err != nil {
		return nil, err
	}
	var result []Service
	for _, s := range services {
		if s.Status != "deleted" && s.GripCoefficient >= minGrip {
			result = append(result, s)
		}
	}
	return result, nil
}
