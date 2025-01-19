package utils

func StringOrPanic(s string) string {
	if s == "" {
		panic("String is empty")
	}

	return s
}
