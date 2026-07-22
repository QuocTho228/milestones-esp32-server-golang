package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	nested "github.com/antonfisher/nested-logrus-formatter"
	log "github.com/sirupsen/logrus"
)

const (
	TYPE_HTTP = 1
)

func init() {
	// Không thiết lập đầu ra mặc định, để ứng dụng tự quyết định
	log.SetFormatter(Formatter(false)) // Mặc định không dùng màu
}

// SetOutput thiết lập đích ghi log
func SetOutput(out *os.File) {
	log.SetOutput(out)
}

// SetLevel thiết lập mức độ log
func SetLevel(level log.Level) {
	log.SetLevel(level)
}

// UseStdout sử dụng đầu ra chuẩn (standard output)
func UseStdout() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(Formatter(true))
}

/*
func getUserInfo(ctx *gin.Context) int {
	if data, ok := ctx.Get("uid"); ok {
		if uid, ok := data.(int); ok {
			return uid
		}
	}
	return 0
}
*/

// getCaller lấy thông tin của bên gọi thực tế (bỏ qua lớp bao bọc của logger)
func getCaller() (string, int) {
	// Bỏ qua call stack của thư viện log để lấy đúng bên gọi thực tế
	// Theo call stack: code người dùng -> logger.Info -> addCallerField -> getCaller -> runtime.Caller
	// Vì vậy cần bỏ qua 3 tầng mới đến được vị trí gọi thực tế
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown", 0
	}
	// Trích xuất tên file (không kèm đường dẫn)
	shortFile := filepath.Base(file)
	return shortFile, line
}

// addCallerField thêm thông tin bên gọi vào field của log
func addCallerField() *log.Entry {
	file, line := getCaller()
	return log.WithField("caller", fmt.Sprintf("%s:%d", file, line))
}

func Info(args ...interface{}) {
	addCallerField().Info(args...)
}

func Error(args ...interface{}) {
	addCallerField().Error(args...)
}

func Debug(args ...interface{}) {
	addCallerField().Debug(args...)
}

func Warn(args ...interface{}) {
	addCallerField().Warn(args...)
}

func Fatal(args ...interface{}) {
	addCallerField().Fatal(args...)
}

func Infof(format string, args ...interface{}) {
	addCallerField().Infof(format, args...)
}

func Errorf(format string, args ...interface{}) {
	addCallerField().Errorf(format, args...)
}

func Debugf(format string, args ...interface{}) {
	addCallerField().Debugf(format, args...)
}

func Warnf(format string, args ...interface{}) {
	addCallerField().Warnf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	addCallerField().Fatalf(format, args...)
}

func Log(args ...interface{}) *log.Entry {
	fields := log.Fields{}
	lenArgs := len(args)
	for i := 0; i < lenArgs; i = i + 2 {
		var key string
		var ok bool
		if key, ok = args[i].(string); !ok {
			continue
		}

		if i <= lenArgs-2 {
			fields[key] = args[i+1]
			continue
		}
		fields[key] = ""
	}

	// Thêm thông tin bên gọi
	// Trong chuỗi gọi hàm Log cũng cần điều chỉnh số tầng
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}
	shortFile := filepath.Base(file)
	fields["caller"] = fmt.Sprintf("%s:%d", shortFile, line)

	log.SetFormatter(Formatter(true))
	return log.WithFields(fields)
}

func Formatter(isConsole bool) *nested.Formatter {
	fmtter := &nested.Formatter{
		FieldsOrder:      []string{"time", "level", "caller", "msg"},
		HideKeys:         true,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		CallerFirst:      true,
		NoUppercaseLevel: true,
		ShowFullLevel:    true,
		//NoFieldsSpace:    true,
		// Vô hiệu hóa định dạng caller mặc định, vì chúng ta đã thêm field caller tùy chỉnh
		CustomCallerFormatter: func(frame *runtime.Frame) string {
			return ""
		},
	}
	if isConsole {
		fmtter.NoColors = false
	} else {
		fmtter.NoColors = true
	}
	return fmtter
}

// DebugStack dùng để debug call stack của log, in ra thông tin tất cả các bên gọi trong chuỗi gọi hiện tại
func DebugStack() {
	for i := 0; i < 5; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		shortFile := filepath.Base(file)
		log.Infof("ngăn xếp gọi[%d]: %s:%d", i, shortFile, line)
	}
}