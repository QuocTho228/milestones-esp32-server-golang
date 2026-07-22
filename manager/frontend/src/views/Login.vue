<template>
  <div class="login-container">
    <div class="login-shell">
      <section class="login-hero">
        <div class="login-brand">
          <img class="login-brand-logo" :src="appLogo" alt="Hệ thống quản lý Milestones" />
          <div>
            <strong>Hệ thống quản lý Milestones</strong>
            <span>Nền tảng quản lý thiết bị và dịch vụ AI</span>
          </div>
        </div>
        <p class="login-eyebrow">MILESTONES CONTROL CENTER</p>
        <h1>Milestones AI Go</h1>
        <p>
          Quản lý thống nhất các tác nhân, giọng nói, cơ sở tri ​​thức và quyền truy cập dịch vụ cho phép các thiết bị,
          mô hình và khả năng giọng nói chạy cộng tác trong cùng một môi trường làm việc.
        </p>
        <div class="login-meta">
          <span class="apple-chip is-primary">Điều phối tác nhân AI</span>
          <span class="apple-chip">Truy cập thiết bị</span>
          <span class="apple-chip">Nhận dạng giọng nói và kho tri thức</span>
          <span class="apple-chip">MCP / OpenClaw</span>
          <span class="apple-chip">Gọi MCP</span>
          <span class="apple-chip">Chủ động gửi giọng nói</span>
        </div>
      </section>

      <el-card class="login-card">
        <template #header>
          <div class="card-header">
            <div>
              <p class="card-eyebrow">WELCOME BACK</p>
              <h2>Đăng nhập hoặc tạo tài khoản</h2>
            </div>
          </div>
        </template>

        <el-tabs v-model="activeTab" class="login-tabs">
          <el-tab-pane label="Đăng nhập" name="login">
            <el-form ref="loginFormRef" :model="loginForm" :rules="loginRules" label-position="top">
              <el-form-item label="Tên người dùng" prop="username">
                <el-input v-model="loginForm.username" placeholder="Vui lòng nhập tên người dùng" />
              </el-form-item>
              <el-form-item label="Mật khẩu" prop="password">
                <el-input
                  v-model="loginForm.password"
                  type="password"
                  placeholder="Vui lòng nhập mật khẩu"
                  @keyup.enter="handleLogin"
                />
              </el-form-item>
              <div v-if="loginCaptchaEnabled" class="captcha-strip">
                <div class="captcha-copy">
                  <span class="captcha-label">Xác minh người máy</span>
                  <strong>{{ loginCaptchaPrompt || 'Đang tạo câu hỏi...' }}</strong>
                  <p>Đơn giản là một bài toán cộng trừ, ngăn chặn việc đăng nhập hàng loạt bằng script.</p>
                </div>
                <el-button link type="primary" :loading="loginCaptchaLoading" @click="refreshLoginCaptcha">
                  Thay đổi câu hỏi
                </el-button>
              </div>
              <el-form-item v-if="loginCaptchaEnabled" label="Kết quả phép tính" prop="captchaAnswer">
                <el-input
                  v-model="loginForm.captchaAnswer"
                  inputmode="numeric"
                  placeholder="Vui lòng nhập kết quả phép tính"
                  @keyup.enter="handleLogin"
                />
              </el-form-item>
              <el-form-item>
                <el-button
                  type="primary"
                  :loading="loading"
                  :disabled="loginCaptchaEnabled && (loginCaptchaLoading || !loginForm.captchaId)"
                  @click="handleLogin"
                  style="width: 100%"
                >
                  Đăng nhập
                </el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>

          <el-tab-pane label="Đăng ký" name="register">
            <el-form ref="registerFormRef" :model="registerForm" :rules="registerRules" label-position="top">
              <el-form-item label="Tên người dùng" prop="username">
                <el-input v-model="registerForm.username" placeholder="Vui lòng nhập tên người dùng" />
              </el-form-item>
              <el-form-item label="Email" prop="email">
                <el-input v-model="registerForm.email" placeholder="Vui lòng nhập email" />
              </el-form-item>
              <el-form-item label="Mật khẩu" prop="password">
                <el-input v-model="registerForm.password" type="password" placeholder="Vui lòng nhập mật khẩu" />
              </el-form-item>
              <el-form-item label="Xác nhận mật khẩu" prop="confirmPassword">
                <el-input
                  v-model="registerForm.confirmPassword"
                  type="password"
                  placeholder="Vui lòng xác nhận mật khẩu"
                  @keyup.enter="handleRegister"
                />
              </el-form-item>
              <div class="captcha-strip">
                <div class="captcha-copy">
                  <span class="captcha-label">Xác minh người máy</span>
                  <strong>{{ registerCaptchaPrompt || 'Đang tạo câu hỏi...' }}</strong>
                  <p>Hoàn thành phép tính đơn giản trước khi gửi đăng ký.</p>
                </div>
                <el-button link type="primary" :loading="registerCaptchaLoading" @click="refreshRegisterCaptcha">
                  Thay đổi câu hỏi
                </el-button>
              </div>
              <el-form-item label="Kết quả tính toán" prop="captchaAnswer">
                <el-input
                  v-model="registerForm.captchaAnswer"
                  inputmode="numeric"
                  placeholder="Vui lòng nhập kết quả phép tính"
                  @keyup.enter="handleRegister"
                />
              </el-form-item>
              <el-form-item>
                <el-button
                  type="primary"
                  :loading="loading"
                  :disabled="registerCaptchaLoading || !registerForm.captchaId"
                  @click="handleRegister"
                  style="width: 100%"
                >
                  Đăng ký
                </el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>
        </el-tabs>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { useAuthStore } from '../stores/auth';
import api from '../utils/api';
import { getPostLoginRedirectPath } from '../utils/authRedirect';
import { checkNeedsSetup } from '../utils/setupStatus';
import appLogo from '@/assets/brand/app-logo.webp';

const router = useRouter();
const authStore = useAuthStore();

const activeTab = ref('login');
const loading = ref(false);
const loginFormRef = ref();
const registerFormRef = ref();
const loginCaptchaPrompt = ref('');
const registerCaptchaPrompt = ref('');
const loginCaptchaLoading = ref(false);
const registerCaptchaLoading = ref(false);
const loginCaptchaEnabled = ref(true);

const loginForm = reactive({
  username: '',
  password: '',
  captchaId: '',
  captchaAnswer: '',
});

const registerForm = reactive({
  username: '',
  email: '',
  password: '',
  confirmPassword: '',
  captchaId: '',
  captchaAnswer: '',
});

const loginRules = {
  username: [{ required: true, message: 'Vui lòng nhập tên người dùng của bạn', trigger: 'blur' }],
  password: [{ required: true, message: 'Vui lòng nhập mật khẩu', trigger: 'blur' }],
  captchaAnswer: [
    {
      validator: (rule, value, callback) => {
        if (!loginCaptchaEnabled.value || String(value || '').trim()) {
          callback();
          return;
        }
        callback(new Error('Hãy nhập kết quả tính toán'));
      },
      trigger: 'blur',
    },
  ],
};

const registerRules = {
  username: [{ required: true, message: 'Vui lòng nhập tên người dùng', trigger: 'blur' }],
  email: [
    { required: true, message: 'Vui lòng nhập địa chỉ email', trigger: 'blur' },
    { type: 'email', message: 'Vui lòng nhập đúng định dạng email', trigger: 'blur' },
  ],
  password: [
    { required: true, message: 'Vui lòng nhập mật khẩu', trigger: 'blur' },
    { min: 6, message: 'Độ dài mật khẩu không được ít hơn 6 ký tự', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: 'Vui lòng xác nhận mật khẩu của bạn', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== registerForm.password) {
          callback(new Error('Hai mật khẩu không khớp nhau'));
        } else {
          callback();
        }
      },
      trigger: 'blur',
    },
  ],
  captchaAnswer: [{ required: true, message: 'Vui lòng nhập kết quả phép tính', trigger: 'blur' }],
};

const fetchCaptcha = async (form, promptRef, loadingRef, formRef) => {
  loadingRef.value = true;
  try {
    const { data } = await api.get('/captcha/challenge', { silentError: true });
    form.captchaId = data.captchaId;
    form.captchaAnswer = '';
    promptRef.value = data.prompt;
    formRef?.value?.clearValidate?.(['captchaAnswer']);
  } catch (error) {
    form.captchaId = '';
    form.captchaAnswer = '';
    promptRef.value = 'Tải câu hỏi không thành công, vui lòng thử lại với một câu hỏi khác';
  } finally {
    loadingRef.value = false;
  }
};

const clearLoginCaptcha = () => {
  loginForm.captchaId = '';
  loginForm.captchaAnswer = '';
  loginCaptchaPrompt.value = '';
  loginFormRef.value?.clearValidate?.(['captchaAnswer']);
};

const loadLoginCaptchaStatus = async () => {
  try {
    const { data } = await api.get('/captcha/status', { silentError: true });
    loginCaptchaEnabled.value = data?.enabled !== false;
  } catch (error) {
    loginCaptchaEnabled.value = true;
  }

  if (!loginCaptchaEnabled.value) {
    clearLoginCaptcha();
  }
};

const refreshLoginCaptcha = async () => {
  if (!loginCaptchaEnabled.value) {
    clearLoginCaptcha();
    return;
  }
  await fetchCaptcha(loginForm, loginCaptchaPrompt, loginCaptchaLoading, loginFormRef);
};

const refreshRegisterCaptcha = async () => {
  await fetchCaptcha(registerForm, registerCaptchaPrompt, registerCaptchaLoading, registerFormRef);
};

const handleLogin = async () => {
  if (!loginFormRef.value) return;

  try {
    await loginFormRef.value.validate();
  } catch {
    return;
  }

  if (loginCaptchaEnabled.value && !loginForm.captchaId) {
    ElMessage.error('Quá trình xác minh người dùng không thành công. Vui lòng thử câu hỏi khác');
    await refreshLoginCaptcha();
    return;
  }

  loading.value = true;
  const credentials = {
    username: loginForm.username,
    password: loginForm.password,
  };
  if (loginCaptchaEnabled.value) {
    credentials.captchaId = loginForm.captchaId;
    credentials.captchaAnswer = loginForm.captchaAnswer.trim();
  }
  const result = await authStore.login(credentials);
  loading.value = false;

  if (result.success) {
    ElMessage.success('Đăng nhập thành công');
    router.push(getPostLoginRedirectPath(authStore.user));
  } else {
    ElMessage.error(result.message);
    if (loginCaptchaEnabled.value) {
      await refreshLoginCaptcha();
    }
  }
};

const handleRegister = async () => {
  if (!registerFormRef.value) return;

  try {
    await registerFormRef.value.validate();
  } catch {
    return;
  }

  if (!registerForm.captchaId) {
    ElMessage.error('Quá trình xác minh người dùng không thành công. Vui lòng thử câu hỏi khác');
    await refreshRegisterCaptcha();
    return;
  }

  loading.value = true;
  const result = await authStore.register({
    username: registerForm.username,
    email: registerForm.email,
    password: registerForm.password,
    captchaId: registerForm.captchaId,
    captchaAnswer: registerForm.captchaAnswer.trim(),
  });
  loading.value = false;

  if (result.success) {
    ElMessage.success('Đăng ký thành công, vui lòng đăng nhập');
    activeTab.value = 'login';
    Object.assign(registerForm, {
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
      captchaId: '',
      captchaAnswer: '',
    });
    await Promise.all([
      loginCaptchaEnabled.value ? refreshLoginCaptcha() : Promise.resolve(),
      refreshRegisterCaptcha(),
    ]);
  } else {
    ElMessage.error(result.message);
    await refreshRegisterCaptcha();
  }
};

// Kiểm tra trạng thái hệ thống, nếu chưa khởi tạo thì chuyển đến trang hướng dẫn
const checkSystemStatus = async () => {
  try {
    if (await checkNeedsSetup()) {
      router.push('/setup');
    }
  } catch (error) {
    console.error('Kiểm tra trạng thái hệ thống thất bại:', error);
  }
};

onMounted(async () => {
  checkSystemStatus();
  await loadLoginCaptchaStatus();
  Promise.allSettled([loginCaptchaEnabled.value ? refreshLoginCaptcha() : Promise.resolve(), refreshRegisterCaptcha()]);
});
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.login-shell {
  width: min(1120px, 100%);
  display: grid;
  grid-template-columns: minmax(0, 1fr) 420px;
  gap: 24px;
  align-items: center;
}

.login-hero {
  padding: 28px;
}

.login-brand {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 24px;
  padding: 10px 14px 10px 10px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.74);
  border: 1px solid rgba(255, 255, 255, 0.86);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.08);
}

.login-brand-logo {
  width: 58px;
  height: 58px;
  border-radius: 20px;
  object-fit: cover;
  box-shadow: 0 12px 24px rgba(0, 122, 255, 0.18);
}

.login-brand strong,
.login-brand span {
  display: block;
}

.login-brand strong {
  color: var(--apple-text);
  font-size: 18px;
  line-height: 1.25;
}

.login-brand span {
  margin-top: 3px;
  color: var(--apple-text-secondary);
  font-size: 13px;
}

.login-eyebrow,
.card-eyebrow {
  margin: 0 0 8px;
  color: var(--apple-primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.login-hero h1 {
  margin: 0;
  font-size: 48px;
  line-height: 1.02;
  letter-spacing: -0.05em;
}

.login-hero p {
  margin: 16px 0 0;
  max-width: 520px;
  color: var(--apple-text-secondary);
  line-height: 1.8;
}

.login-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 22px;
}

.login-card {
  border-radius: 30px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(255, 255, 255, 0.9);
  box-shadow: var(--apple-shadow-lg);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h2 {
  margin: 0;
  color: var(--apple-text);
  font-size: 25px;
  letter-spacing: -0.03em;
}

.login-tabs {
  margin-top: 8px;
}

.captcha-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  padding: 14px 16px;
  border-radius: 18px;
  border: 1px solid var(--apple-border);
  background: rgba(247, 248, 250, 0.92);
}

.captcha-copy {
  min-width: 0;
}

.captcha-copy strong {
  display: block;
  color: var(--apple-text);
  font-size: 18px;
  letter-spacing: -0.02em;
}

.captcha-copy p {
  margin: 6px 0 0;
  color: var(--apple-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.captcha-label {
  display: inline-block;
  margin-bottom: 6px;
  color: var(--apple-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

@media (max-width: 960px) {
  .login-shell {
    grid-template-columns: 1fr;
  }

  .login-hero {
    padding: 8px 0;
  }

  .login-hero h1 {
    font-size: 38px;
  }

  .captcha-strip {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
