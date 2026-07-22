package util

import (
	"bytes"
	"strings"
	"sync"
	"unicode"
)

var (
	// punctuationMap bản đồ các dấu câu kết thúc câu và dấu ngắt câu
	// LƯU Ý: các ký tự trong map này là ký tự dấu câu thực tế được dùng để so khớp logic,
	// KHÔNG được thay đổi/dịch các ký tự này, nếu không sẽ làm hỏng chức năng tách câu.
	punctuationMap = map[rune]bool{
		'。':  true,
		'？':  true,
		'！':  true,
		'；':  true,
		'：':  true,
		'\n': true,
		'.':  true,
		'?':  true,
		'!':  true,
		';':  true,
		':':  true,
	}

	// firstPunctuation bản đồ dấu câu dùng khi xử lý lần đầu (bao gồm cả dấu phẩy)
	firstPunctuation = map[rune]bool{
		'，':  true,
		',':  true,
		'。':  true,
		'？':  true,
		'！':  true,
		'；':  true,
		'：':  true,
		'\n': true,
		'.':  true,
		'?':  true,
		'!':  true,
		';':  true,
		':':  true,
	}

	// sentenceEndPunctuation các dấu câu kết thúc câu
	sentenceEndPunctuation = []rune{'.', '。', '!', '！', '?', '？', '\n'}

	// sentencePausePunctuation các dấu câu ngắt nghỉ trong câu (có thể dùng làm điểm ngắt cho câu dài)
	sentencePausePunctuation = []rune{',', '，', ';', '；', ':', '：'}

	// builderPool pool đối tượng dùng để tái sử dụng
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// runeSlicePool pool slice dùng để lưu trữ kết quả
	runeSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]rune, 0, 1024)
			return &slice
		},
	}
)

// IsSentenceEndPunctuation kiểm tra một ký tự có phải là dấu câu kết thúc câu hay không
func IsSentenceEndPunctuation(r rune) bool {
	for _, p := range sentenceEndPunctuation {
		if r == p {
			return true
		}
	}
	return false
}

// IsSentencePausePunctuation kiểm tra một ký tự có phải là dấu câu ngắt nghỉ trong câu hay không
func IsSentencePausePunctuation(r rune) bool {
	for _, p := range sentencePausePunctuation {
		if r == p {
			return true
		}
	}
	return false
}

// IsNumberWithDot kiểm tra chuỗi có phải ở định dạng số kèm dấu chấm hay không (như "1.", "2." v.v.)
func IsNumberWithDot(s string) bool {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 || trimmed[len(trimmed)-1] != '.' {
		return false
	}

	for i := 0; i < len(trimmed)-1; i++ {
		if !unicode.IsDigit(rune(trimmed[i])) {
			return false
		}
	}
	return true
}

// ExtractCompleteSentences trích xuất các câu hoàn chỉnh từ văn bản
// Trả về slice các câu hoàn chỉnh và phần nội dung chưa hoàn thành còn lại
func ExtractCompleteSentences(text string) ([]string, string) {
	if text == "" {
		return []string{}, ""
	}

	var sentences []string
	var currentSentence bytes.Buffer

	runes := []rune(text)
	lastIndex := len(runes) - 1

	for i, r := range runes {
		currentSentence.WriteRune(r)

		// Kiểm tra câu đã kết thúc chưa
		if IsSentenceEndPunctuation(r) {
			// Nếu là dấu câu kết thúc câu
			sentence := strings.TrimSpace(currentSentence.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			currentSentence.Reset()
		} else if i == lastIndex {
			// Nếu là ký tự cuối cùng nhưng không phải dấu câu kết thúc câu, giữ lại trong remaining
			break
		}
	}

	// Câu chưa hoàn thành hiện tại được trả về dưới dạng remaining
	remaining := currentSentence.String()
	return sentences, strings.TrimSpace(remaining)
}

// isNumberPrefix sử dụng kiểm tra ký tự nhanh thay cho regex, để xác định có phải là tiền tố số thứ tự hay không
func isNumberPrefix(text []rune, pos int) bool {
	if pos <= 0 || text[pos] != '.' {
		return false
	}

	// Tìm ngược lại đầu dòng hoặc ký tự xuống dòng
	start := pos - 1
	digitCount := 0
	foundDigit := false

	// Bỏ qua các ký tự khoảng trắng trước dấu chấm
	for start >= 0 && (text[start] == ' ' || text[start] == '\t') {
		start--
	}

	// Đếm số chữ số
	for start >= 0 && text[start] >= '0' && text[start] <= '9' {
		digitCount++
		foundDigit = true
		if digitCount > 3 { // Quá 3 chữ số thì không phải là số thứ tự hợp lệ
			return false
		}
		start--
	}

	// Kiểm tra ký tự trước chữ số có phải là khoảng trắng hoặc đầu dòng không
	if start >= 0 && text[start] != ' ' && text[start] != '\t' && text[start] != '\n' {
		return false
	}

	return foundDigit
}

// trimSpaceRunes loại bỏ khoảng trắng ở đầu và cuối
func trimSpaceRunes(text []rune) []rune {
	start, end := 0, len(text)-1

	for start <= end && (text[start] == ' ' || text[start] == '\t' || text[start] == '\n') {
		start++
	}

	for end >= start && (text[end] == ' ' || text[end] == '\t' || text[end] == '\n') {
		end--
	}

	if start > end {
		return nil
	}
	return text[start : end+1]
}

func isDigitAdjacentColon(text []rune, pos int) bool {
	if pos < 0 || pos >= len(text) {
		return false
	}

	colon := text[pos]
	if colon != ':' && colon != '：' {
		return false
	}

	if pos == 0 || !unicode.IsDigit(text[pos-1]) {
		return false
	}

	if pos == len(text)-1 {
		return true
	}

	return unicode.IsDigit(text[pos+1])
}

// findLastPunctuation tìm dấu câu cuối cùng theo hướng từ sau ra trước
func findLastPunctuation(text []rune, separatorMap map[rune]bool) int {
	lastPos := -1
	for i := len(text) - 1; i >= 0; i-- {
		// Kiểm tra có phải dấu câu không
		if separatorMap[text[i]] {
			// Nếu là dấu chấm, kiểm tra xem có phải một phần của số thứ tự không
			if text[i] == '.' && isNumberPrefix(text, i) {
				continue
			}
			if isDigitAdjacentColon(text, i) {
				continue
			}
			return i
		}
	}
	return lastPos
}

// findNextSplitPoint tìm điểm chia tiếp theo
func findNextSplitPoint(text []rune, startPos int, maxLen int, separatorMap map[rune]bool) int {
	// Tính vị trí kết thúc tìm kiếm
	endPos := startPos + maxLen
	if endPos > len(text) {
		endPos = len(text)
	}

	// Tìm kiếm từ trước ra sau
	for i := startPos; i < endPos; i++ {
		// Kiểm tra có phải ký tự xuống dòng không, đồng thời kiểm tra dòng tiếp theo có phải số thứ tự không
		if text[i] == '\n' {
			nextPos := i + 1
			// Bỏ qua khoảng trắng
			for nextPos < endPos && (text[nextPos] == ' ' || text[nextPos] == '\t') {
				nextPos++
			}
			// Kiểm tra có phải bắt đầu bằng số thứ tự không
			if nextPos < endPos-2 && text[nextPos] >= '0' && text[nextPos] <= '9' {
				return i
			}
			continue
		}

		// Dùng map để kiểm tra có phải dấu câu không
		if separatorMap[text[i]] {
			if isDigitAdjacentColon(text, i) {
				continue
			}
			return i
		}
	}

	// Nếu không tìm thấy trong phạm vi maxLen, thử tìm trong phạm vi lớn hơn
	if endPos < len(text) {
		for i := endPos; i < len(text); i++ {
			if text[i] == '\n' {
				return i
			}
			if separatorMap[text[i]] {
				if isDigitAdjacentColon(text, i) {
					continue
				}
				return i
			}
		}
	}

	return -1
}

// ExtractSmartSentences trích xuất câu thông minh
// text: văn bản cần xử lý
// minLen: độ dài câu tối thiểu
// maxLen: độ dài câu tối đa
// isFirst: có phải lần xử lý đầu tiên hay không (khi xử lý lần đầu cho phép dùng dấu phẩy làm dấu phân cách)
func ExtractSmartSentences(text string, minLen, maxLen int, isFirst bool) (sentences []string, remaining string) {
	// Khi isFirst là true, nới lỏng để dùng dấu phẩy làm dấu phân cách
	separatorMap := punctuationMap
	if isFirst {
		separatorMap = firstPunctuation
	}
	// Cấp phát trước một dung lượng slice hợp lý
	estimatedCount := len(text) / 50
	if estimatedCount < 10 {
		estimatedCount = 10
	}
	sentences = make([]string, 0, estimatedCount)

	// Chuyển đổi sang rune slice một lần
	currentRunes := []rune(text)
	startPos := 0

	// Lấy đối tượng tái sử dụng từ pool
	builder := builderPool.Get().(*strings.Builder)
	defer builderPool.Put(builder)
	builder.Grow(maxLen * 2)

	// Lấy rune slice tạm thời
	tempRunesPtr := runeSlicePool.Get().(*[]rune)
	tempRunes := (*tempRunesPtr)[:0]
	defer runeSlicePool.Put(tempRunesPtr)

	for startPos < len(currentRunes) {
		// Bỏ qua khoảng trắng ở đầu
		for startPos < len(currentRunes) && (currentRunes[startPos] == ' ' || currentRunes[startPos] == '\t' || currentRunes[startPos] == '\n') {
			startPos++
		}

		if startPos >= len(currentRunes) {
			break
		}

		// Tìm điểm chia tiếp theo
		splitPos := findNextSplitPoint(currentRunes, startPos, maxLen, separatorMap)
		if splitPos == -1 {
			// Không tìm thấy điểm chia, coi phần văn bản còn lại là remaining
			segment := trimSpaceRunes(currentRunes[startPos:])
			if len(segment) > 0 {
				remaining = string(segment)
			}
			break
		}

		// Trích xuất đoạn hiện tại
		builder.Reset()
		tempRunes = tempRunes[:0]

		// Thu thập và xử lý đoạn hiện tại
		segment := trimSpaceRunes(currentRunes[startPos : splitPos+1])

		// Kiểm tra đoạn có đạt độ dài tối thiểu và kết thúc bằng dấu câu không
		if len(segment) >= minLen && separatorMap[segment[len(segment)-1]] {
			sentences = append(sentences, string(segment))
		} else {
			// Nếu không thỏa điều kiện, thêm vào remaining
			if len(segment) > 0 {
				if len(remaining) > 0 {
					remaining += " "
				}
				remaining += string(segment)
			}
		}

		startPos = splitPos + 1
	}

	return sentences, remaining
}

// ContainsSentenceSeparator kiểm tra chuỗi có chứa dấu phân cách (dấu kết thúc câu hoặc dấu ngắt nghỉ) hay không
func ContainsSentenceSeparator(s string, isFirst bool) bool {
	separatorMap := punctuationMap
	if isFirst {
		separatorMap = firstPunctuation
	}

	runes := []rune(s)
	for i, r := range runes {
		if !separatorMap[r] {
			continue
		}
		if isDigitAdjacentColon(runes, i) {
			continue
		}
		return true
	}

	return false
}