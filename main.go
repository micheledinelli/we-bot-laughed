package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	_ "embed"

	"github.com/rs/zerolog/log"

	"github.com/syumai/workers"
	"github.com/syumai/workers/cloudflare/cron"
	_ "github.com/syumai/workers/cloudflare/d1"
	"github.com/syumai/workers/cloudflare/fetch"

	"github.com/micheledinelli/we-bot-laughed/telegram"
	"github.com/micheledinelli/we-bot-laughed/webhook"
)

//go:embed db/query_current_chapter.sql
var queryCurrentChapter string

//go:embed db/query_insert_chapter.sql
var queryInsertChapter string

//go:embed db/query_select_users.sql
var querySelectUsers string

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", webhook.Handler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("I am laughing!"))
	})

	workers.ServeNonBlock(mux)
	cron.ScheduleTaskNonBlock(scrapeTask)

	workers.Ready()

	select {
	case <-workers.Done():
	case <-cron.Done():
	}
}

func scrapeTask(ctx context.Context) error {
	e, err := cron.NewEvent(ctx)
	log.Info().Msgf("Scrape task started at %d", e.ScheduledTime.Unix())

	var scrapeUrl = "https://tcbonepiecechapters.com/"

	db, err := sql.Open("d1", "DB")
	if err != nil {
		log.Error().Err(err).Msg("failed to connect to database")
		return err
	}
	defer db.Close()

	var currentChapterNum int64
	var currentUrl string
	err = db.QueryRowContext(ctx, queryCurrentChapter).Scan(&currentChapterNum, &currentUrl)
	if err != nil && err != sql.ErrNoRows {
		log.Error().Err(err).Msg("failed to query current chapter")
		return err
	}

	client := fetch.NewClient()
	r, err := fetch.NewRequest(ctx, http.MethodGet, scrapeUrl, nil)
	if err != nil {
		log.Error().Err(err).Msg("http client network error")
		return err
	}

	res, err := client.Do(r, nil)

	if res.Status != "200 OK" {
		log.Error().Str("status", res.Status).Msg("unexpected status code")
		return nil
	}
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)
	bodyString := string(bodyBytes)

	var pattern string = `/chapters/\d+/one-piece-chapter-` +
		regexp.QuoteMeta(strconv.FormatInt(currentChapterNum+1, 10))
	re := regexp.MustCompile(pattern)
	match := re.FindString(bodyString)

	if match != "" {
		log.Info().Msgf("scrape task finished at %d, match: %s", e.ScheduledTime.Unix(), match)

		url := scrapeUrl + match
		newChapter := currentChapterNum + 1

		_, err = db.ExecContext(ctx, queryInsertChapter, newChapter, url)

		rows, err := db.QueryContext(ctx, querySelectUsers)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var chatID int64
				rows.Scan(&chatID)
				telegram.SendTelegramMessage(
					ctx,
					chatID,
					fmt.Sprintf("New chapter is out! Check it out at %s", url))
			}
		}
	}

	return nil
}
