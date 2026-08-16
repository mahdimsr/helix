package notification

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type KavenegarService struct {
	APIKey string
	Client *http.Client
}

func NewKavenegarService(apiKey string) *KavenegarService {
	return &KavenegarService{
		APIKey: apiKey,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *KavenegarService) SendVerificationSMS(receptor, template string, tokens map[string]string) {
	go func() {
		baseURL := fmt.Sprintf("https://api.kavenegar.com/v1/%s/verify/lookup.json", s.APIKey)

		params := url.Values{}
		params.Add("receptor", receptor)
		params.Add("template", template)

		for key, value := range tokens {
			params.Add(key, value)
		}

		reqURL := baseURL + "?" + params.Encode()

		resp, err := s.Client.Get(reqURL)
		if err != nil {
			log.Printf("[SMS ERROR] Failed to send SMS to %s: %v", receptor, err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[SMS ERROR] Failed to read response body for %s: %v", receptor, err)
			return
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("[SMS ERROR] API returned status %d for %s. Body: %s", resp.StatusCode, receptor, string(body))
			return
		}

		log.Printf("[SMS SUCCESS] SMS sent successfully to %s.", receptor)
	}()
}
