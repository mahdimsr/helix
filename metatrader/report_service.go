package metatrader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type ReportService struct {
	baseURL string
	client  *http.Client
}

type ReportResponse struct {
	Available bool  `json:"available"`
	ChatID    int64 `json:"chatId"`
}

func NewReportService(baseURL string) *ReportService {
	return &ReportService{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Check: period مثل "1h" و "3h" و "24h" است و به‌صورت query param ?period=1h فرستاده می‌شود
func (r *ReportService) Check(period string) (ReportResponse, error) {
	var out ReportResponse

	u, err := url.Parse(r.baseURL)
	if err != nil {
		return out, err
	}
	q := u.Query()
	q.Set("period", period)
	u.RawQuery = q.Encode()

	resp, err := r.client.Get(u.String())
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("report status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}
