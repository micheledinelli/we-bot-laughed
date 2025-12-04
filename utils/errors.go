package utils

import "errors"

var ErrorDatabaseConnection error = errors.New("failed to connect to database")
var ErrorDatabaseDisConnection error = errors.New("failed to disconnect to database")
var ErrorDatabasePing error = errors.New("failed to ping database")

var ErrorMongoFind error = errors.New("failed to find in database")
var ErrorMongoFindOne error = errors.New("failed to find one in database")
var ErrorMongoInsertOne error = errors.New("failed to insert one in database")
var ErrorMongoDeleteOne error = errors.New("failed to delete one in database")
var ErrorMongoUpdateOne error = errors.New("failed to update one in database")

var ErrorMongoCursor error = errors.New("failed to decode cursor")
var ErrorGenericMongoCursor error = errors.New("failed to decode cursor")
var ErrorChatId = errors.New("failed to get chat id or convert it to int64")

var ErrorLoadingEnv = errors.New("failed to load .env")
var ErrorCreatingBot = errors.New("failed to create telegram bot")

type EnvVariabaleNotFoundError error

func NewEnvVariableNotFoundError(k string) error {
	return errors.New("Couldn't find env variable for key " + k)
}
