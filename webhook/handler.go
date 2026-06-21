package webhook

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	_ "embed"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/micheledinelli/we-bot-laughed/telegram"
	"github.com/rs/zerolog/log"
)

func Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("failed to read webhook body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal telegram update")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if update.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	chatID := update.Message.Chat.ID
	text := update.Message.Text
	user := update.Message.From
	message := update.Message

	log.Info().
		Int64("user_id", int64(user.ID)).
		Str("username", user.UserName).
		Str("first_name", user.FirstName).
		Str("last_name", user.LastName).
		Int64("chat_id", message.Chat.ID).
		Int("message_id", message.MessageID).
		Str("text", text).
		Msg("Received a message")

	db, err := sql.Open("d1", "DB")
	if err != nil {
		log.Error().Err(err).Msg("db connection error in webhook")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer db.Close()

	switch text {
	case "/start":
		_, err = db.ExecContext(ctx, "INSERT INTO users (chat_id) VALUES (?)", chatID)
		if err != nil {
			log.Error().Err(err).Msg("failed to add user")
		}

		var latestChapterURL string
		if err = db.QueryRowContext(ctx,
			"SELECT url FROM chapters WHERE id = (SELECT MAX(id) FROM chapters)").Scan(&latestChapterURL); err != nil {
			log.Error().Err(err).Msg("failed to query latest chapter URL")
			return
		}

		telegram.SendTelegramMessage(ctx,
			chatID,
			fmt.Sprintf("Started watching for one piece updates for you. You will be notified when a new chapter is out. In the meantime, you can check the latest chapter at %s", latestChapterURL))

	case "/stop":
		_, err = db.ExecContext(ctx, "DELETE FROM users WHERE chat_id = ?", chatID)
		if err != nil {
			log.Error().Err(err).Msg("failed to remove user")
		}
		telegram.SendTelegramMessage(ctx, chatID, "See ya!")
	}

	w.WriteHeader(http.StatusOK)
}
