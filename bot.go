package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"op-bot/db"
	"op-bot/utils"
)

var (
	startMessage = "Started watching for one piece updates for you. You will be notified when a new chapter is out."
	bot          *tgbotapi.BotAPI
	log          = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

func main() {
	var err error
	var m *db.Mongo

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	if err = godotenv.Load(); err != nil {
		log.Error("Couldn't load .env file", "error", err)
		panic(err)
	}

	token := utils.StringEnvOrPanic("TELEGRAM_HTTP_API_TOKEN")
	scrapeUrl := utils.StringEnvOrPanic("SCRAPE_URL")
	mongoUri := utils.StringEnvOrPanic("MONGO_URI")

	if m, err = db.InitDatabase(ctx, mongoUri); err != nil {
		// slog.Fatalf("Couldn't init database: %v", err)
		slog.Error("Couldn't init database", "error", err)
		panic(err)
	}

	defer func() {
		if err = m.Client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	if bot, err = tgbotapi.NewBotAPI(token); err != nil {
		log.Error("Couldn't create bot", "error", err)
		panic(err)
	}

	bot.Debug = utils.BoolEnvOrPanic("DEBUG")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	go receiveUpdates(ctx, updates, m)

	c := make(chan string)

	go scrapeForOpContent(ctx, scrapeUrl, m, c)
	go sendUpdates(ctx, m, c)

	log.Info("Bot started", "username", bot.Self.UserName)

	if err = http.ListenAndServe(":8080", nil); err != nil {
		log.Error("Couldn't start HTTP server", "error", err)
		panic(err)
	}

	cancel()
}

func scrapeForOpContent(ctx context.Context, url string, m *db.Mongo, c chan string) {
	for {
		var err error
		chapter := &utils.Chapter{}

		if chapter, err = m.GetLatestChapter(); err != nil {
			log.Error("Couldn't get latest chapter", "error", err)
		}

		var pattern string = `/chapters/\d+/one-piece-chapter-` +
			regexp.QuoteMeta(strconv.FormatInt(chapter.ChapterNumber, 10))
		re := regexp.MustCompile(pattern)

		select {
		case <-ctx.Done():
			return

		default:
			var err error
			var resp *http.Response
			var client http.Client

			if resp, err = client.Get(url); err != nil {
				panic(err)
			}

			if resp.StatusCode == http.StatusOK {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Error("Couldn't read response body", "error", err)
				}
				bodyString := string(bodyBytes)

				match := re.FindString(bodyString)

				if match != "" {
					c <- url + match

					m.UpdateLatestChapter(chapter.ChapterNumber, url+match)
				}
			}

		}

		// Check every hour
		time.Sleep(time.Hour)
	}
}

func receiveUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel, m *db.Mongo) {
	for {
		select {
		case <-ctx.Done():
			return

		case update := <-updates:
			handleUpdate(update, m)
		}
	}
}

func sendUpdates(ctx context.Context, m *db.Mongo, c chan string) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			url := <-c
			var chatIds *[]int64
			var err error

			if chatIds, err = m.GetUsers(); err != nil {
				log.Error("Couldn't get users", "error", err)
				continue
			}

			for _, chatId := range *chatIds {
				sendOpIsOutMsg(chatId, url)
			}
		}
	}
}

func handleUpdate(update tgbotapi.Update, m *db.Mongo) {
	switch {
	case update.Message != nil:
		handleMessage(update.Message, m)

	default:
		log.Info("Received an update that is not a message",
			"update_id", update.UpdateID,
			"update", update)
	}
}

func handleMessage(message *tgbotapi.Message, m *db.Mongo) {
	user := message.From
	text := message.Text

	if user == nil {
		return
	}

	var err error

	if strings.HasPrefix(text, "/") {
		err = handleCommand(message.Chat.ID, text, m)
	}

	if err != nil {
		log.Error("Error handling message",
			"chat_id", message.Chat.ID,
			"message_id", message.MessageID,
			"error", err)
	}
}

func handleCommand(chatId int64, command string, m *db.Mongo) error {
	var err error

	log.Info("Received command",
		"chat_id", chatId,
		"command", command)

	switch command {
	case "/start":
		if err = sendStartMessage(chatId); err != nil {
			log.Error("Couldn't send start message",
				"chat_id", chatId,
				"error", err)
		}

		if err = m.AddUser(chatId); err != nil {
			log.Error("Couldn't add user",
				"chat_id", chatId,
				"error", err)
		}

		if err = sendLatestChapter(chatId, m); err != nil {
			log.Error("Couldn't send latest chapter",
				"chat_id", chatId,
				"error", err)
		}

	case "/stop":
		if err = m.RemoveUser(chatId); err != nil {
			log.Error("Couldn't remove user",
				"chat_id", chatId,
				"error", err)
		}
	}

	return err
}

func sendStartMessage(chatId int64) error {
	msg := tgbotapi.NewMessage(chatId, startMessage)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := bot.Send(msg)
	return err
}

func sendOpIsOutMsg(chatId int64, url string) error {
	tokens := strings.Split(url, "-")
	chapter := tokens[len(tokens)-1]
	str := fmt.Sprintf("One Piece %s is out at %s", chapter, url)
	msg := tgbotapi.NewMessage(chatId, str)
	_, err := bot.Send(msg)
	return err
}

func sendLatestChapter(chatId int64, m *db.Mongo) error {
	var err error
	chapter := &utils.Chapter{}

	if chapter, err = m.GetLatestChapter(); err != nil {
		log.Error("Couldn't get latest chapter: %v", err)
	}

	str := fmt.Sprintf("Meanwhile you can read the latest chapter of One Piece at %s", chapter.Url)
	msg := tgbotapi.NewMessage(chatId, str)
	_, err = bot.Send(msg)
	return err
}
