<template>
  <div class="test-page">
    <h1>Trang kiểm tra</h1>
    <p>Nếu bạn thấy trang này, hệ thống định tuyến đang hoạt động bình thường.</p>
    <div class="debug-info">
      <h3>Thông tin gỡ lỗi:</h3>
      <p>Đường dẫn hiện tại: {{ $route.path }}</p>
      <p>Trạng thái xác thực người dùng: {{ authStore.isAuthenticated }}</p>
      <p>Thông tin người dùng: {{ authStore.user }}</p>
      <p>Là quản trị viên: {{ authStore.isAdmin }}</p>
    </div>
    <div class="actions">
      <el-button @click="$router.push('/login')">Đến trang đăng nhập</el-button>
      <el-button @click="$router.push('/dashboard')">Đến bảng điều khiển</el-button>
      <el-button @click="$router.push('/agents')">Đến danh sách trợ lý AI</el-button>
    </div>
  </div>
</template>

<script setup>
import { useAuthStore } from '../stores/auth';

const authStore = useAuthStore();
</script>

<style scoped>
.test-page {
  padding: 24px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: var(--apple-shadow-md);
}

.test-page h1 {
  margin: 0 0 12px;
  color: var(--apple-text);
}

.test-page > p {
  margin: 0;
  color: var(--apple-text-secondary);
}

.debug-info {
  margin: 20px 0;
  padding: 16px 18px;
  background: rgba(248, 250, 252, 0.92);
  border: 1px solid rgba(229, 229, 234, 0.72);
  border-radius: 18px;
  color: var(--apple-text-secondary);
}

.debug-info h3 {
  margin: 20px 0;
  color: var(--apple-text);
}

.debug-info p {
  margin: 8px 0;
}

.actions {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.actions .el-button {
  margin-right: 0;
}
</style>
