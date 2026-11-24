package asyncclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client — HTTP-клиент асинхронного сервиса (NestJS), который с задержкой пересчитывает углы поворота 3D модели
type Client struct {
	url  string
	http *http.Client
}

func New(url string) *Client {
	return &Client{url: url, http: &http.Client{Timeout: 3 * time.Second}}
}

// rotationRequest — тело запроса к асинхронному сервису: услуга и текущее положение её 3D модели
type rotationRequest struct {
	ID     uint    `json:"id"`
	AngleX float64 `json:"angle_x"`
	AngleY float64 `json:"angle_y"`
}

// RequestRotation — просит асинхронный сервис пересчитать углы услуги; сервис отвечает сразу,
// а результат присылает позже отдельным запросом PUT /api/tire-conditions/{id}/angles
func (c *Client) RequestRotation(id uint, angleX, angleY float64) error {
	body, err := json.Marshal(rotationRequest{ID: id, AngleX: angleX, AngleY: angleY})
	if err != nil {
		return err
	}

	resp, err := c.http.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("async service: unexpected status %d", resp.StatusCode)
	}
	return nil
}
