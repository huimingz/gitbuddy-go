package lang

// Language represents supported output languages
type Language string

const (
	English            Language = "en"
	ChineseSimplified  Language = "zh"
	ChineseTraditional Language = "zh-tw"
	Japanese           Language = "ja"
	Korean             Language = "ko"
)

// String returns the string representation of the language
func (l Language) String() string {
	return string(l)
}

// IsValid checks if the language is valid
func (l Language) IsValid() bool {
	switch l {
	case English, ChineseSimplified, ChineseTraditional, Japanese, Korean:
		return true
	default:
		return false
	}
}

// DisplayName returns the display name of the language
func (l Language) DisplayName() string {
	switch l {
	case English:
		return "English"
	case ChineseSimplified:
		return "中文（简体）"
	case ChineseTraditional:
		return "中文（繁體）"
	case Japanese:
		return "日本語"
	case Korean:
		return "한국어"
	default:
		return string(l)
	}
}

// DefaultLanguage returns the default language
func DefaultLanguage() Language {
	return English
}

// ParseLanguage parses a string to a Language
func ParseLanguage(s string) Language {
	l := Language(s)
	if l.IsValid() {
		return l
	}
	return DefaultLanguage()
}
