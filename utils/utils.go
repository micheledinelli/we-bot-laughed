package utils

import "os"

func StringEnvOrPanic(k string) string {
	s := os.Getenv(k)
	if s == "" {
		panic(NewEnvVariableNotFoundError(k))
	}

	return s
}

func BoolEnvOrPanic(k string) bool {
	s := os.Getenv(k)
	if s == "" {
		panic(NewEnvVariableNotFoundError(k))
	}

	return s == "true"
}
