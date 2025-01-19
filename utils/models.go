package utils

type Chapter struct {
	ChapterNumber int64  `bson:"chapter_number"`
	Url           string `bson:"latest_url"`
}
