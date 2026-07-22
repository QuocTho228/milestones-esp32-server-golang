package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AudioStorage công cụ lưu trữ file âm thanh
type AudioStorage struct {
	BasePath string
	MaxSize  int64
}

// NewAudioStorage tạo mới instance lưu trữ âm thanh
func NewAudioStorage(basePath string, maxSize int64) *AudioStorage {
	// Đảm bảo thư mục gốc tồn tại
	if err := os.MkdirAll(basePath, 0755); err != nil {
		panic(fmt.Sprintf("Không thể tạo thư mục lưu trữ âm thanh: %v", err))
	}

	return &AudioStorage{
		BasePath: basePath,
		MaxSize:  maxSize,
	}
}

// SaveAudioFile lưu file âm thanh
// userID: ID người dùng
// groupID: ID nhóm vân giọng nói
// uuid: định danh UUID
// fileName: tên file gốc
// fileData: dữ liệu file
// Trả về: đường dẫn lưu file, kích thước file, lỗi (nếu có)
func (s *AudioStorage) SaveAudioFile(userID uint, groupID uint, uuid, fileName string, fileData io.Reader) (string, int64, error) {
	// Xây dựng đường dẫn lưu trữ: storage/speakers/{user_id}/{group_id}/{uuid}.wav
	dirPath := filepath.Join(s.BasePath, fmt.Sprintf("%d", userID), fmt.Sprintf("%d", groupID))

	// Đảm bảo thư mục tồn tại
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", 0, fmt.Errorf("Tạo thư mục thất bại: %v", err)
	}

	// Xây dựng đường dẫn file (dùng UUID làm tên file, giữ nguyên phần mở rộng)
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".wav" // Phần mở rộng mặc định
	}
	filePath := filepath.Join(dirPath, fmt.Sprintf("%s%s", uuid, ext))

	// Tạo file
	file, err := os.Create(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("Tạo file thất bại: %v", err)
	}
	defer file.Close()

	// Ghi dữ liệu vào file (giới hạn kích thước)
	limitedReader := io.LimitReader(fileData, s.MaxSize)
	written, err := io.Copy(file, limitedReader)
	if err != nil {
		os.Remove(filePath) // Xóa file đã ghi dở
		return "", 0, fmt.Errorf("Ghi file thất bại: %v", err)
	}

	// Kiểm tra kích thước file
	if written >= s.MaxSize {
		os.Remove(filePath)
		return "", 0, fmt.Errorf("Kích thước file vượt quá giới hạn: %d byte", s.MaxSize)
	}

	return filePath, written, nil
}

// SaveVoiceCloneAudioFile lưu file âm thanh dùng để nhân bản giọng nói
func (s *AudioStorage) SaveVoiceCloneAudioFile(userID uint, uuid, fileName string, fileData io.Reader) (string, int64, error) {
	dirPath := filepath.Join(s.BasePath, "voice_clones", fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", 0, fmt.Errorf("Tạo thư mục thất bại: %v", err)
	}

	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".wav"
	}
	filePath := filepath.Join(dirPath, fmt.Sprintf("%s%s", uuid, ext))

	file, err := os.Create(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("Tạo file thất bại: %v", err)
	}
	defer file.Close()

	limitedReader := io.LimitReader(fileData, s.MaxSize)
	written, err := io.Copy(file, limitedReader)
	if err != nil {
		os.Remove(filePath)
		return "", 0, fmt.Errorf("Ghi file thất bại: %v", err)
	}
	if written >= s.MaxSize {
		os.Remove(filePath)
		return "", 0, fmt.Errorf("Kích thước file vượt quá giới hạn: %d byte", s.MaxSize)
	}

	return filePath, written, nil
}

// DeleteAudioFile xóa file âm thanh
func (s *AudioStorage) DeleteAudioFile(filePath string) error {
	if filePath == "" {
		return nil
	}

	// Kiểm tra file có tồn tại hay không
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File không tồn tại, không cần xóa
	}

	return os.Remove(filePath)
}

// GetAudioFile lấy file âm thanh
func (s *AudioStorage) GetAudioFile(filePath string) (*os.File, error) {
	return os.Open(filePath)
}

// FileExists kiểm tra file có tồn tại hay không
func (s *AudioStorage) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}