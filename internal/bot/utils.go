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

func NormalizePhoneNumber(raw string) string {
	digitsOnly := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")

	if strings.HasPrefix(digitsOnly, "8") {
		digitsOnly = "7" + digitsOnly[1:]
	}

	if strings.HasPrefix(digitsOnly, "7") && len(digitsOnly) == 11 {
		return digitsOnly
	}

	return digitsOnly
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
	matched, _ := regexp.MatchString(`^7\d{10}$`, phone)

	return matched
}

func GenerateAuthCode() string {
	rand.Seed(time.Now().UnixNano())

	return fmt.Sprintf("%06d", rand.Intn(1000000))
}
