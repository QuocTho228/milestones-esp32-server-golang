package chat

import (
	"strings"
	"unicode"

	"github.com/spf13/viper"
)

// removePunctuation: Xóa dấu câu trong văn bản.
func removePunctuation(text string) string {
	// Tạo một đối tượng strings.Builder.
	var builder strings.Builder
	builder.Grow(len(text))

	for _, r := range text {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// isWakeupWord: Kiểm tra xem văn bản có phải là từ khóa đánh thức hay không.
func isWakeupWord(text string) bool {
	wakeupWords := viper.GetStringSlice("wakeup_words")
	for _, word := range wakeupWords {
		if text == word {
			return true
		}
	}
	return false
}
