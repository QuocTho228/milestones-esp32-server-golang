package webrtc_vad

import (
	"context"
	"sync"
	"testing"
	"time"

	"milestones-esp32-server-golang/internal/util"
)

func TestWebRTCVADPool(t *testing.T) {
	// Tạo cấu hình VAD
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	// Tạo cấu hình pool
	poolConfig := &util.PoolConfig{
		MaxSize:          3,
		MinSize:          1,
		MaxIdle:          2,
		AcquireTimeout:   5 * time.Second,
		IdleTimeout:      1 * time.Minute,
		ValidateOnBorrow: true,
		ValidateOnReturn: true,
	}

	// Tạo resource pool cho VAD
	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		t.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	// Kiểm thử lấy và giải phóng VAD
	vad, err := pool.AcquireVAD()
	if err != nil {
		t.Fatalf("Failed to acquire VAD: %v", err)
	}

	// Kiểm thử chức năng VAD
	testData := make([]float32, 320) // Dữ liệu audio 16kHz trong 20ms
	for i := range testData {
		testData[i] = 0.1 // Điền một số dữ liệu kiểm thử
	}

	active, err := vad.IsVAD(testData)
	if err != nil {
		t.Errorf("VAD detection failed: %v", err)
	}

	t.Logf("VAD result: %v", active)

	// Giải phóng VAD
	err = pool.ReleaseVAD(vad)
	if err != nil {
		t.Errorf("Failed to release VAD: %v", err)
	}

	// Kiểm tra thông tin thống kê
	stats := pool.Stats()
	t.Logf("Pool stats: %+v", stats)
}

func TestWebRTCVADPoolConcurrency(t *testing.T) {
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	poolConfig := &util.PoolConfig{
		MaxSize:        5,
		MinSize:        2,
		MaxIdle:        3,
		AcquireTimeout: 10 * time.Second,
		IdleTimeout:    30 * time.Second,
	}

	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		t.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	// Kiểm thử đồng thời (concurrency)
	numWorkers := 10
	numIterations := 5
	var wg sync.WaitGroup

	testData := make([]float32, 320)
	for i := range testData {
		testData[i] = float32(i%100) / 100.0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Lấy thực thể VAD
				vad, err := pool.AcquireVAD()
				if err != nil {
					t.Errorf("Worker %d iteration %d: Failed to acquire VAD: %v", workerID, j, err)
					return
				}

				// Sử dụng VAD
				_, err = vad.IsVAD(testData)
				if err != nil {
					t.Errorf("Worker %d iteration %d: VAD detection failed: %v", workerID, j, err)
				}

				// Mô phỏng thời gian xử lý
				time.Sleep(10 * time.Millisecond)

				// Giải phóng VAD
				err = pool.ReleaseVAD(vad)
				if err != nil {
					t.Errorf("Worker %d iteration %d: Failed to release VAD: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Kiểm tra thông tin thống kê cuối cùng
	stats := pool.Stats()
	t.Logf("Final pool stats: %+v", stats)
}

func TestWebRTCVADFactory(t *testing.T) {
	config := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	factory := NewWebRTCVADFactory(config)

	// Kiểm thử tạo resource
	resource, err := factory.Create()
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}
	defer resource.Close()

	// Kiểm tra kiểu resource
	vad, ok := resource.(*WebRTCVAD)
	if !ok {
		t.Fatalf("Created resource is not WebRTCVAD type")
	}

	// Kiểm tra cấu hình
	if vad.GetSampleRate() != config.SampleRate {
		t.Errorf("Expected sample rate %d, got %d", config.SampleRate, vad.GetSampleRate())
	}

	if vad.GetMode() != config.Mode {
		t.Errorf("Expected mode %d, got %d", config.Mode, vad.GetMode())
	}

	// Kiểm thử chức năng validate
	if !factory.Validate(resource) {
		t.Error("Factory validation failed for valid resource")
	}

	// Kiểm thử chức năng reset
	err = factory.Reset(resource)
	if err != nil {
		t.Errorf("Factory reset failed: %v", err)
	}

	// Kiểm thử resource còn hợp lệ hay không
	if !resource.IsValid() {
		t.Error("Resource should be valid after reset")
	}
}

func TestWebRTCVADPoolTimeout(t *testing.T) {
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	poolConfig := &util.PoolConfig{
		MaxSize:        1, // Chỉ cho phép một resource
		MinSize:        1,
		MaxIdle:        1,
		AcquireTimeout: 100 * time.Millisecond, // Thời gian timeout ngắn
		IdleTimeout:    1 * time.Minute,
	}

	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		t.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	// Lấy thực thể VAD đầu tiên
	vad1, err := pool.AcquireVAD()
	if err != nil {
		t.Fatalf("Failed to acquire first VAD: %v", err)
	}

	// Thử lấy thực thể VAD thứ hai, sẽ bị timeout
	start := time.Now()
	vad2, err := pool.AcquireVAD()
	elapsed := time.Since(start)

	if err == nil {
		pool.ReleaseVAD(vad2)
		t.Error("Expected timeout error, but got VAD instance")
	}

	if elapsed < 90*time.Millisecond {
		t.Errorf("Expected timeout around 100ms, but got %v", elapsed)
	}

	// Giải phóng VAD đầu tiên
	err = pool.ReleaseVAD(vad1)
	if err != nil {
		t.Errorf("Failed to release VAD: %v", err)
	}

	// Bây giờ phải có thể lấy được VAD
	vad3, err := pool.AcquireVAD()
	if err != nil {
		t.Errorf("Failed to acquire VAD after release: %v", err)
	}
	pool.ReleaseVAD(vad3)
}

// BenchmarkWebRTCVADPool kiểm thử hiệu năng
func BenchmarkWebRTCVADPool(b *testing.B) {
	vadConfig := WebRTCVADConfig{
		SampleRate: 16000,
		Mode:       2,
	}

	poolConfig := &util.PoolConfig{
		MaxSize: 10,
		MinSize: 2,
		MaxIdle: 5,
	}

	pool, err := NewWebRTCVADPool(vadConfig, poolConfig)
	if err != nil {
		b.Fatalf("Failed to create WebRTC VAD pool: %v", err)
	}
	defer pool.Close()

	testData := make([]float32, 320)
	for i := range testData {
		testData[i] = float32(i%100) / 100.0
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			vad, err := pool.AcquireVAD()
			if err != nil {
				b.Errorf("Failed to acquire VAD: %v", err)
				continue
			}

			_, err = vad.IsVAD(testData)
			if err != nil {
				b.Errorf("VAD detection failed: %v", err)
			}

			pool.ReleaseVAD(vad)
		}
	})
}