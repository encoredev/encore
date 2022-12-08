package api

func clampTo64Chars(str string) string {
	if len(str) > 64 {
		return str[:64]
	}
	return str
}
