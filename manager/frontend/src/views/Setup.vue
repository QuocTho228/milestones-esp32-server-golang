<template>
  <div class="setup-container">
    <div class="setup-shell">
      <section class="setup-intro">
        <p class="setup-eyebrow">FIRST RUN EXPERIENCE</p>
        <h1>Hoàn thành việc khởi tạo hệ thống một cách nhẹ nhàng hơn.</h1>
        <p>
          Trước tiên, hãy tạo tài khoản quản trị viên, sau đó nhập bảng điều khiển hợp nhất và trình hướng dẫn cấu hình.
        </p>
      </section>

      <div class="setup-card">
        <div class="setup-header">
          <p class="setup-card-eyebrow">SYSTEM SETUP</p>
          <h1>Khởi tạo hệ thống</h1>
          <p>Chào mừng bạn đến với hệ thống quản lý Milestones, vui lòng hoàn tất cài đặt ban đầu.</p>
        </div>

        <div v-if="!initialized" class="setup-status">
          <div class="loading-spinner" v-if="checking">
            <div class="spinner"></div>
            <p>Đang kiểm tra trạng thái hệ thống...</p>
          </div>

          <div v-else-if="needsSetup" class="setup-form">
            <h2>Tạo tài khoản quản trị viên</h2>
            <p>Vui lòng thiết lập thông tin tài khoản quản trị viên để quản lý hệ thống</p>

            <form @submit.prevent="initializeSystem">
              <div class="form-group">
                <label for="username">Tên người dùng quản trị</label>
                <input
                  id="username"
                  v-model="form.admin_username"
                  type="text"
                  required
                  minlength="3"
                  maxlength="50"
                  placeholder="Vui lòng nhập tên người dùng quản trị viên"
                />
              </div>

              <div class="form-group">
                <label for="email">Email quản trị</label>
                <input
                  id="email"
                  v-model="form.admin_email"
                  type="email"
                  required
                  placeholder="Vui lòng nhập địa chỉ email của quản trị viên"
                />
              </div>

              <div class="form-group">
                <label for="password">Mật khẩu quản trị</label>
                <input
                  id="password"
                  v-model="form.admin_password"
                  type="password"
                  required
                  minlength="6"
                  maxlength="100"
                  placeholder="Vui lòng nhập mật khẩu quản trị viên (ít nhất 6 chữ số)"
                />
              </div>

              <div class="form-group">
                <label for="confirmPassword">Xác nhận mật khẩu</label>
                <input
                  id="confirmPassword"
                  v-model="confirmPassword"
                  type="password"
                  required
                  placeholder="Vui lòng nhập lại mật khẩu"
                />
              </div>

              <div class="error-message" v-if="errorMessage">
                {{ errorMessage }}
              </div>

              <button type="submit" :disabled="initializing" class="setup-btn">
                <span v-if="initializing">Đang khởi tạo...</span>
                <span v-else>Bắt đầu khởi tạo</span>
              </button>
            </form>
          </div>

          <div v-else class="setup-complete">
            <div class="success-icon">
              <span class="success-badge">OK</span>
            </div>
            <h2>Hệ thống đã được khởi tạo</h2>
            <p>Hệ thống đã hoàn tất quá trình khởi tạo, vui lòng sử dụng tài khoản quản trị để đăng nhập</p>
            <router-link to="/login" class="login-btn">Đi đến trang đăng nhập</router-link>
          </div>
        </div>

        <div v-else class="setup-success">
          <div class="success-icon">
            <span class="success-badge">OK</span>
          </div>
          <h2>Khởi tạo thành công!</h2>
          <p>Hệ thống đã được khởi tạo thành công, tài khoản quản trị đã được tạo</p>
          <div class="admin-info">
            <p><strong>Tên người dùng:</strong>{{ adminInfo.username }}</p>
            <p><strong>Email：</strong>{{ adminInfo.email }}</p>
          </div>
          <router-link to="/login" class="login-btn">Đi đến trang đăng nhập</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import api from '@/utils/api';

export default {
  name: 'Setup',
  setup() {
    const router = useRouter();
    const checking = ref(true);
    const needsSetup = ref(false);
    const initialized = ref(false);
    const initializing = ref(false);
    const errorMessage = ref('');

    const form = ref({
      admin_username: '',
      admin_email: '',
      admin_password: '',
    });

    const confirmPassword = ref('');
    const adminInfo = ref({});

    const checkSetupStatus = async () => {
      try {
        checking.value = true;
        const response = await api.get('/setup/status');

        if (response.data.needs_setup) {
          needsSetup.value = true;
        } else {
          // Hệ thống đã khởi tạo, chuyển đến trang đăng nhập
          router.push('/login');
        }
      } catch (error) {
        console.error('Kiểm tra trạng thái hệ thống không thành công:', error);
        errorMessage.value = 'Không kiểm tra được trạng thái hệ thống, vui lòng làm mới trang và thử lại';
      } finally {
        checking.value = false;
      }
    };

    const initializeSystem = async () => {
      // Xác minh xác nhận mật khẩu
      if (form.value.admin_password !== confirmPassword.value) {
        errorMessage.value = 'Mật khẩu nhập hai lần không nhất quán';
        return;
      }

      try {
        initializing.value = true;
        errorMessage.value = '';

        const response = await api.post('/setup/initialize', form.value);

        adminInfo.value = response.data.admin;
        initialized.value = true;
      } catch (error) {
        console.error('Hệ thống khởi tạo thất bại:', error);
        if (error.response?.data?.error) {
          errorMessage.value = error.response.data.error;
        } else {
          errorMessage.value = 'Khởi tạo hệ thống không thành công, vui lòng thử lại';
        }
      } finally {
        initializing.value = false;
      }
    };

    onMounted(() => {
      checkSetupStatus();
    });

    return {
      checking,
      needsSetup,
      initialized,
      initializing,
      errorMessage,
      form,
      confirmPassword,
      adminInfo,
      initializeSystem,
    };
  },
};
</script>

<style scoped>
.setup-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.setup-shell {
  width: min(1100px, 100%);
  display: grid;
  grid-template-columns: minmax(0, 1fr) 460px;
  gap: 24px;
  align-items: center;
}

.setup-intro {
  padding: 20px 8px;
}

.setup-eyebrow,
.setup-card-eyebrow {
  margin: 0 0 8px;
  color: var(--apple-primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.setup-intro h1 {
  margin: 0;
  font-size: 48px;
  line-height: 1.04;
  letter-spacing: -0.05em;
}

.setup-intro p {
  max-width: 560px;
  margin: 16px 0 0;
  color: var(--apple-text-secondary);
  font-size: 15px;
  line-height: 1.8;
}

.setup-card {
  background: rgba(255, 255, 255, 0.88);
  border-radius: 30px;
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: var(--apple-shadow-lg);
  padding: 36px;
  width: 100%;
}

.setup-header {
  margin-bottom: 30px;
}

.setup-header h1 {
  color: var(--apple-text);
  margin-bottom: 10px;
  font-size: 30px;
  letter-spacing: -0.03em;
}

.setup-header p {
  color: var(--apple-text-secondary);
  font-size: 15px;
  line-height: 1.8;
}

.loading-spinner {
  text-align: center;
  padding: 40px 0;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 4px solid rgba(0, 122, 255, 0.12);
  border-top: 4px solid var(--apple-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 20px;
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

.setup-form h2 {
  color: var(--apple-text);
  margin-bottom: 10px;
  text-align: left;
}

.setup-form p {
  color: var(--apple-text-secondary);
  text-align: left;
  margin-bottom: 30px;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  color: var(--apple-text);
  font-weight: 600;
}

.form-group input {
  width: 100%;
  padding: 14px 16px;
  border: 1px solid rgba(229, 229, 234, 0.9);
  border-radius: 16px;
  font-size: 16px;
  background: rgba(255, 255, 255, 0.96);
  transition:
    border-color var(--apple-transition-fast),
    box-shadow var(--apple-transition-fast);
  box-sizing: border-box;
}

.form-group input:focus {
  outline: none;
  border-color: rgba(0, 122, 255, 0.34);
  box-shadow: 0 0 0 4px rgba(0, 122, 255, 0.08);
}

.error-message {
  color: #8a1f19;
  background: rgba(255, 59, 48, 0.08);
  border: 1px solid rgba(255, 59, 48, 0.16);
  padding: 12px;
  border-radius: 16px;
  margin-bottom: 20px;
  font-size: 14px;
}

.setup-btn {
  width: 100%;
  padding: 14px;
  background: linear-gradient(180deg, #2e90ff 0%, #007aff 100%);
  color: white;
  border: none;
  border-radius: 16px;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 14px 28px rgba(0, 122, 255, 0.2);
}

.setup-btn:hover:not(:disabled) {
  transform: translateY(-1px);
}

.setup-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.setup-complete,
.setup-success {
  text-align: center;
  padding: 40px 0;
}

.success-icon {
  display: flex;
  justify-content: center;
  margin-bottom: 20px;
}

.success-badge {
  width: 72px;
  height: 72px;
  border-radius: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--apple-primary);
  font-size: 20px;
  font-weight: 700;
  background: linear-gradient(180deg, #eff6ff 0%, #dcebff 100%);
}

.setup-complete h2,
.setup-success h2 {
  color: var(--apple-text);
  margin-bottom: 10px;
}

.setup-complete p,
.setup-success p {
  color: var(--apple-text-secondary);
  margin-bottom: 30px;
  line-height: 1.7;
}

.admin-info {
  background: rgba(248, 250, 252, 0.9);
  padding: 20px;
  border-radius: 20px;
  border: 1px solid rgba(229, 229, 234, 0.72);
  margin-bottom: 30px;
  text-align: left;
}

.admin-info p {
  margin: 8px 0;
  color: var(--apple-text);
}

.login-btn {
  display: inline-block;
  padding: 12px 24px;
  background: linear-gradient(180deg, #2e90ff 0%, #007aff 100%);
  color: white;
  text-decoration: none;
  border-radius: 16px;
  font-weight: 600;
  box-shadow: 0 14px 28px rgba(0, 122, 255, 0.2);
}

.login-btn:hover {
  transform: translateY(-1px);
}

@media (max-width: 960px) {
  .setup-shell {
    grid-template-columns: 1fr;
  }

  .setup-intro {
    padding: 0 0 8px;
  }

  .setup-intro h1 {
    font-size: 38px;
  }
}
</style>
