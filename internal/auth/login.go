package auth

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxLoginLength = 50
	fallbackLogin  = "user"
)

var (
	loginPattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

	translitTable = map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh",
		'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
		'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts",
		'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu",
		'я': "ya",
	}
)

func SuggestLogin(name Name) string {
	login := translit(name.Last)

	if name.First != "" {
		first, _ := utf8.DecodeRuneInString(name.First)
		if initial := translit(string(first)); initial != "" {
			login += "." + initial
		}
	}

	login = strings.Trim(login, ".")
	if login == "" {
		return fallbackLogin
	}

	if len(login) > maxLoginLength {
		login = login[:maxLoginLength]
	}

	return login
}

func translit(word string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(word) {
		if latin, ok := translitTable[r]; ok {
			b.WriteString(latin)
			continue
		}

		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
	}

	return b.String()
}

func normalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

func validLogin(login string) bool {
	return len(login) <= maxLoginLength && loginPattern.MatchString(login)
}
