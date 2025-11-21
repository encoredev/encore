package cmdutil

type Language string

const (
	LanguageGo Language = "go"
	LanguageTS Language = "ts"
)

var AllLanguages = []Language{
	LanguageGo,
	LanguageTS,
}

func LanguageFlagValues() []string {
	result := make([]string, 0, len(AllLanguages))
	for _, r := range AllLanguages {
		result = append(result, string(r))
	}
	return result
}

func (lang Language) Display() string {
	switch lang {
	case LanguageGo:
		return "Go"
	case LanguageTS:
		return "TypeScript"
	default:
		return string(lang)
	}
}

func (lang Language) SelectPrompt() string {
	return "Select language for your application"
}
