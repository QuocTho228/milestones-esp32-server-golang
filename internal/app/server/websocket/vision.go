package websocket

import (
	"io"
	"net/http"
	"strings"
	"milestones-esp32-server-golang/internal/app/server/chat"
	log "milestones-esp32-server-golang/logger"

	"github.com/spf13/viper"
)

// handleVisionAPI xử lý API nhận diện hình ảnh
func (s *WebSocketServer) handleVisionAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Warnf("Phương thức yêu cầu nhận diện hình ảnh không đúng: %s", r.Method)
		http.Error(w, "Chỉ hỗ trợ yêu cầu POST", http.StatusMethodNotAllowed)
		return
	}

	//Lấy Device-Id và Client-Id từ header
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")
	_ = clientId
	if deviceId == "" {
		log.Errorf("Yêu cầu nhận diện hình ảnh thiếu Device-Id")
		http.Error(w, "Thiếu Device-Id", http.StatusBadRequest)
		return
	}
	log.Infof("Yêu cầu nhận diện hình ảnh deviceId=%s", deviceId)

	if viper.GetBool("vision.enable_auth") {

		//Lấy Bearer token từ header Authorization
		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			log.Errorf("Yêu cầu nhận diện hình ảnh thiếu Authorization deviceId=%s", deviceId)
			http.Error(w, "Thiếu Authorization", http.StatusBadRequest)
			return
		}
		authToken = strings.TrimPrefix(authToken, "Bearer ")

		err := chat.VisvionAuth(authToken)
		if err != nil {
			log.Errorf("Xác thực nhận diện hình ảnh thất bại deviceId=%s err=%v", deviceId, err)
			http.Error(w, "Xác thực nhận diện hình ảnh thất bại", http.StatusUnauthorized)
			return
		}
		log.Infof("Xác thực nhận diện hình ảnh thành công deviceId=%s", deviceId)
	}

	// Giải mã form multipart, tối đa 10MB
	question := r.FormValue("question")
	if question == "" {
		log.Warnf("Yêu cầu nhận diện hình ảnh thiếu question deviceId=%s", deviceId)
		http.Error(w, "Thiếu tham số question", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Errorf("Yêu cầu nhận diện hình ảnh thiếu file hoặc đọc thất bại deviceId=%s err=%v", deviceId, err)
		http.Error(w, "Thiếu tham số file hoặc đọc file thất bại", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Đọc file nhận diện hình ảnh thất bại deviceId=%s err=%v", deviceId, err)
		http.Error(w, "Đọc file thất bại", http.StatusInternalServerError)
		return
	}

	file.Close()
	log.Infof("Nhận diện hình ảnh đã nhận file deviceId=%s filename=%s size=%d question=%s", deviceId, header.Filename, len(fileBytes), question)

	result, err := chat.HandleVllm(deviceId, fileBytes, question)
	if err != nil {
		log.Errorf("Nhận diện hình ảnh thất bại deviceId=%s err=%v", deviceId, err)
		http.Error(w, "Nhận diện hình ảnh thất bại", http.StatusInternalServerError)
		return
	}

	log.Infof("Nhận diện hình ảnh thành công deviceId=%s resultLen=%d", deviceId, len(result))
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}