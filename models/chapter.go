package models

type Chapter struct {
	ChapterNumber int64  `bson:"chapter_number" json:"chapter_number"`
	Url           string `bson:"latest_url" json:"latest_url"`
}
