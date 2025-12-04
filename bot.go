package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/micheledinelli/we-bot-laughed/db"
	"github.com/micheledinelli/we-bot-laughed/utils"
)

var (
	startMessage = "Started watching for one piece updates for you. You will be notified when a new chapter is out."
	bot          *tgbotapi.BotAPI
)

func main() {
	var err error
	var mongo *db.Mongo

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	if err = godotenv.Load(); err != nil {
		log.Fatal().
			Err(err).
			Msgf(utils.ErrorLoadingEnv.Error())
	}

	token := utils.StringEnvOrPanic("TELEGRAM_HTTP_API_TOKEN")
	scrapeUrl := utils.StringEnvOrPanic("SCRAPE_URL")
	mongoUri := utils.StringEnvOrPanic("MONGO_URI")

	if mongo, err = db.InitDatabase(ctx, mongoUri); err != nil {
		cancel()
		log.Fatal().Err(err).Msg(utils.ErrorDatabaseConnection.Error())
	}

	defer func() {
		if err = mongo.Client.Disconnect(ctx); err != nil {
			cancel()
			log.Fatal().Err(err).Msg(utils.ErrorDatabaseDisConnection.Error())
		}
	}()

	if bot, err = tgbotapi.NewBotAPI(token); err != nil {
		cancel()
		log.Fatal().Err(err).Msg(utils.ErrorCreatingBot.Error())
	}

	bot.Debug = utils.BoolEnvOrPanic("DEBUG")
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	c := make(chan string)

	go receiveUpdates(ctx, updates, mongo)
	go scrapeTCB(ctx, scrapeUrl, mongo, c)
	// go scrapeOPScans(ctx, mongo, c)
	go sendUpdates(ctx, mongo, c)

	log.Printf("bot %s started", bot.Self.UserName)

	<-ctx.Done()
}

func scrapeTCB(ctx context.Context, url string, m *db.Mongo, c chan string) {
	for {
		var err error
		chapter := &utils.Chapter{}

		if chapter, err = m.GetLatestChapter(); err != nil {
			log.Error().Err(err).Msg("Couldn't get the latest chapter")
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
				log.Error().Err(err).Msgf("http protocol error host: %s", url)
			}

			if resp.StatusCode == http.StatusOK {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Error().Err(err).Msg("couldn't read response body")
				}

				bodyString := string(bodyBytes)
				match := re.FindString(bodyString)
				if match != "" {
					c <- url + match
					m.UpdateLatestChapter(chapter.ChapterNumber, url+match)
				}
			}
		}

		// Check every half an hour
		time.Sleep(time.Hour / 2)
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
				log.Error().Err(err).Msg("couldn't get users")
				continue
			}

			for _, chatId := range *chatIds {
				if err = sendOpIsOutMsg(chatId, url); err != nil {
					log.Error().Err(err).Msg("error sending op is out msg")
				}
			}
		}
	}
}

func handleUpdate(update tgbotapi.Update, m *db.Mongo) {
	switch {
	case update.Message != nil:
		handleMessage(update.Message, m)

	default:
		log.Info().
			Int("update_id", update.UpdateID).
			Str("update", update.Message.Text).
			Msg("Received an update that is not a message")
	}
}

func handleMessage(message *tgbotapi.Message, m *db.Mongo) {
	var err error
	user := message.From
	text := message.Text

	if user == nil {
		return
	}

	log.Info().
		Int64("user_id", int64(user.ID)).
		Str("username", user.UserName).
		Str("first_name", user.FirstName).
		Str("last_name", user.LastName).
		Int64("chat_id", message.Chat.ID).
		Int("message_id", message.MessageID).
		Str("text", text).
		Msg("Received a message")

	if strings.HasPrefix(text, "/") {
		err = handleCommand(message.Chat.ID, text, m)
	}

	if err != nil {
		log.Error().
			Int64("chat_id", message.Chat.ID).
			Int("message_id", message.MessageID).
			Err(err).
			Msg("Error handling message")
	}
}

func handleCommand(chatId int64, command string, m *db.Mongo) error {
	var err error

	log.Error().
		Int64("chat_id", chatId).
		Str("command", command).
		Msg("received command")

	switch command {
	case "/start":
		if err = sendStartMessage(chatId); err != nil {
			log.Error().
				Int64("chat_id", chatId).
				Err(err).
				Msg("couldn't send start message")
		}

		if err = m.AddUser(chatId); err != nil {
			log.Error().
				Int64("chat_id", chatId).
				Err(err).
				Msg("couldn't add user")
		}

		if err = sendLatestChapter(chatId, m); err != nil {
			log.Error().
				Int64("chat_id", chatId).
				Err(err).
				Msg("couldn't send latest chapter")
		}

	case "/stop":
		if err = m.RemoveUser(chatId); err != nil {
			log.Error().
				Int64("chat_id", chatId).
				Err(err).
				Msg("couldn't remove user")
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
		log.Error().
			Err(err).
			Msg("couldn't get latest chapter")
		return err
	}

	str := fmt.Sprintf("Meanwhile you can read the latest chapter of One Piece at %s", chapter.Url)
	msg := tgbotapi.NewMessage(chatId, str)
	_, err = bot.Send(msg)
	return err
}
