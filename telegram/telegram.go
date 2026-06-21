package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/syumai/workers/cloudflare"
	"github.com/syumai/workers/cloudflare/fetch"
)

func SendTelegramMessage(ctx context.Context, chatID int64, text string) error {
	token := cloudflare.Getenv("TELEGRAM_HTTP_API_TOKEN")
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	payload, err := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal telegram payload")
		return err
	}

	log.Info().Msgf("Sending message to %d: %s", chatID, text)

	bodyReader := bytes.NewReader(payload)
	req, err := fetch.NewRequest(ctx, http.MethodPost, apiURL, bodyReader)
	if err != nil {
		log.Error().Err(err).Msg("failed to create fetch request for telegram")
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := fetch.NewClient()
	res, err := client.Do(req, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed executing fetch payload to telegram API")
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(res.Body)
		log.Error().
			Int("status_code", res.StatusCode).
			Str("response", string(respBody)).
			Msg("telegram API returned an unexpected error status")
		return fmt.Errorf("telegram api error: status %d", res.StatusCode)
	}

	return nil
}
