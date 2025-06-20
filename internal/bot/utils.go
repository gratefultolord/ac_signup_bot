package bot

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

func NormalizeText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ToLower(text)

	return text
}

func IsValidDate(date string) (time.Time, bool) {
	matched, _ := regexp.MatchString(`\d{2}\.\d{2}\.\d{4}$`, date)
	if !matched {
		return time.Time{}, false
	}

	parsed, err := time.Parse("02.01.2006", date)
	if err != nil {
		return time.Time{}, false
	}

	return parsed, true
}

func IsValidPhoneNumber(phone string) bool {
	matched, _ := regexp.MatchString(`^\+\d{10,15}$`, phone)

	return matched
}

func GenerateAuthCode() string {
	rand.Seed(time.Now().UnixNano())

	return fmt.Sprintf("%06d", rand.Intn(1000000))
}
