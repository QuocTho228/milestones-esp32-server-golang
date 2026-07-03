<template>
  <div class="config-page">
    <div class="page-actions">
      <el-input
        v-model="searchKeyword"
        placeholder="Tìm kiếm người dùng..."
        style="width: 200px"
        prefix-icon="Search"
        clearable
      />
      <el-button type="primary" @click="openAddDialog">
        <el-icon><Plus /></el-icon>
        Thêm người dùng
      </el-button>
    </div>

    <!-- Bảng danh sách người dùng -->
    <el-table :data="filteredUserList" v-loading="tableLoading" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="Tên đăng nhập" width="150" />
      <el-table-column prop="email" label="Email" width="200" />
      <el-table-column prop="role" label="Vai trò" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'">
            {{ row.role === 'admin' ? 'Quản trị viên' : 'Người dùng' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="Thời gian tạo" width="180">
        <template #default="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="Thao tác" width="360">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">Chỉnh sửa</el-button>
          <el-button size="small" type="success" @click="openQuotaDialog(row)" :disabled="row.role === 'admin'"
            >Hạn mức nhân bản giọng</el-button
          >
          <el-button size="small" type="warning" @click="openResetPasswordDialog(row)"> Đặt lại mật khẩu </el-button>
          <el-button size="small" type="danger" @click="handleDeleteUser(row)" :disabled="row.role === 'admin'">
            Xóa
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Thêm/Chỉnh sửaNgười dùngdialog -->
    <el-dialog
      v-model="userDialogVisible"
      :title="isEditMode ? 'Chỉnh sửaNgười dùng' : 'Thêm người dùng'"
      width="500px"
      @close="resetUserForm"
    >
      <el-form ref="userFormRef" :model="userForm" :rules="userFormRules" label-width="80px">
        <el-form-item label="Tên đăng nhập" prop="username">
          <el-input v-model="userForm.username" :disabled="isEditMode" placeholder="Vui lòng nhậpTên đăng nhập" />
        </el-form-item>

        <el-form-item label="Email" prop="email">
          <el-input v-model="userForm.email" placeholder="Vui lòng nhậpEmail" />
        </el-form-item>

        <el-form-item v-if="!isEditMode" label="Mật khẩu" prop="password">
          <el-input
            v-model="userForm.password"
            type="password"
            placeholder="Vui lòng nhập mật khẩu (ít nhất 6 ký tự)"
            show-password
          />
        </el-form-item>

        <el-form-item label="Vai trò" prop="role">
          <el-select v-model="userForm.role" placeholder="Vui lòng chọnVai trò" style="width: 100%">
            <el-option label="Người dùng" value="user" />
            <el-option label="Quản trị viên" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="userDialogVisible = false">Hủy</el-button>
        <el-button type="primary" @click="handleUserSubmit" :loading="userSubmitLoading">
          {{ isEditMode ? 'Lưu' : 'Thêm' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Đặt lại mật khẩudialog -->
    <el-dialog v-model="resetPasswordDialogVisible" title="Đặt lại mật khẩu" width="400px" @close="resetPasswordForm">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordFormRules" label-width="80px">
        <el-form-item label="Người dùng">
          <el-input v-model="currentUser.username" disabled />
        </el-form-item>

        <el-form-item label="mớiMật khẩu" prop="newPassword">
          <el-input
            v-model="passwordForm.newPassword"
            type="password"
            placeholder="Vui lòng nhập mật khẩu mới (ít nhất 6 ký tự)"
            show-password
          />
        </el-form-item>

        <el-form-item label="Xác nhận mật khẩu" prop="confirmPassword">
          <el-input
            v-model="passwordForm.confirmPassword"
            type="password"
            placeholder="Vui lòng nhập lại mật khẩu mới"
            show-password
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">Hủy</el-button>
        <el-button type="primary" @click="handleResetPassword" :loading="resetPasswordLoading">
          Xác nhận đặt lại
        </el-button>
      </template>
    </el-dialog>

    <!-- giọng nóiHạn mức nhân bản giọngdialog -->
    <el-dialog
      v-model="quotaDialogVisible"
      :title="`giọng nóiHạn mức nhân bản giọng - ${quotaUser.username || ''}`"
      width="900px"
      @close="resetQuotaDialog"
    >
      <div class="quota-hint">
        Phân bổ số lần nhân bản theo cấu hình TTS: -1 không giới hạn, 0 cấm tạo, số nguyên dương là số lần tối đa.
      </div>
      <el-table :data="quotaRows" v-loading="quotaLoading" style="margin-top: 12px">
        <el-table-column prop="tts_config_name" label="TTSTên cấu hình" min-width="180" />
        <el-table-column prop="tts_config_id" label="TTS Config ID" min-width="180" />
        <el-table-column prop="provider" label="Provider" width="120" />
        <el-table-column label="Đã dùng" width="100">
          <template #default="{ row }">{{ row.used_count }}</template>
        </el-table-column>
        <el-table-column label="Còn lại" width="100">
          <template #default="{ row }">{{ row.remaining_count < 0 ? 'Không giới hạn' : row.remaining_count }}</template>
        </el-table-column>
        <el-table-column label="Số lần tối đa" width="180">
          <template #default="{ row }">
            <el-input-number
              v-model="row.max_count"
              :min="-1"
              :step="1"
              :precision="0"
              controls-position="right"
              style="width: 140px"
            />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="quotaDialogVisible = false">Hủy</el-button>
        <el-button type="primary" :loading="quotaSaving" @click="saveQuotaSettings">Lưuhạn mức</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import api from '../../utils/api';

// Trạng thái dữ liệu
const userList = ref([]);
const tableLoading = ref(false);
const userDialogVisible = ref(false);
const resetPasswordDialogVisible = ref(false);
const userSubmitLoading = ref(false);
const resetPasswordLoading = ref(false);
const quotaDialogVisible = ref(false);
const quotaLoading = ref(false);
const quotaSaving = ref(false);
const quotaRows = ref([]);
const quotaUser = ref({});
const quotaOriginalMaxMap = ref({});
const isEditMode = ref(false);
const currentUser = ref({});
const searchKeyword = ref('');

// Thuộc tính tính toán
const filteredUserList = computed(() => {
  if (!searchKeyword.value) {
    return userList.value;
  }
  return userList.value.filter(
    (user) =>
      user.username.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
      user.email.toLowerCase().includes(searchKeyword.value.toLowerCase()),
  );
});

// Tham chiếu form
const userFormRef = ref();
const passwordFormRef = ref();

// Người dùngdữ liệu form
const userForm = reactive({
  username: '',
  email: '',
  password: '',
  role: '',
});

// Mật khẩudữ liệu form
const passwordForm = reactive({
  newPassword: '',
  confirmPassword: '',
});

// Người dùngformquy tắc xác thực
const userFormRules = {
  username: [{ required: true, message: 'Vui lòng nhậpTên đăng nhập', trigger: 'blur' }],
  email: [
    { required: true, message: 'Vui lòng nhập Email', trigger: 'blur' },
    { type: 'email', message: 'Vui lòng nhập đúng định dạng Email', trigger: 'blur' },
  ],
  password: [
    { required: true, message: 'Vui lòng nhậpMật khẩu', trigger: 'blur' },
    { min: 6, message: 'Mật khẩuđộ dài không được ít hơn6 ký tự', trigger: 'blur' },
  ],
  role: [{ required: true, message: 'Vui lòng chọnVai trò', trigger: 'change' }],
};

// Mật khẩuformquy tắc xác thực
const passwordFormRules = {
  newPassword: [
    { required: true, message: 'Vui lòng nhậpmớiMật khẩu', trigger: 'blur' },
    { min: 6, message: 'Mật khẩuđộ dài không được ít hơn6 ký tự', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: 'Vui lòng xác nhận mật khẩu', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error('hai lần nhậpMật khẩukhông khớp'));
        } else {
          callback();
        }
      },
      trigger: 'blur',
    },
  ],
};

// TảiNgười dùngdanh sách
const loadUserList = async () => {
  tableLoading.value = true;
  try {
    const response = await api.get('/admin/users');
    userList.value = response.data.data || [];
  } catch (error) {
    ElMessage.error('TảiNgười dùngdanh sáchthất bại');
  } finally {
    tableLoading.value = false;
  }
};

// MởThêm người dùngdialog
const openAddDialog = () => {
  isEditMode.value = false;
  userDialogVisible.value = true;
};

// MởChỉnh sửaNgười dùngdialog
const openEditDialog = (user) => {
  isEditMode.value = true;
  currentUser.value = user;
  userForm.username = user.username;
  userForm.email = user.email;
  userForm.role = user.role;
  userDialogVisible.value = true;
};

// đặt lạiNgười dùngform
const resetUserForm = () => {
  userForm.username = '';
  userForm.email = '';
  userForm.password = '';
  userForm.role = '';
  currentUser.value = {};
  if (userFormRef.value) {
    userFormRef.value.resetFields();
  }
};

// Xử lýNgười dùnggửi
const handleUserSubmit = async () => {
  if (!userFormRef.value) return;

  try {
    await userFormRef.value.validate();
    userSubmitLoading.value = true;

    if (isEditMode.value) {
      // Chỉnh sửaNgười dùng
      await api.put(`/admin/users/${currentUser.value.id}`, {
        email: userForm.email,
        role: userForm.role,
      });
      ElMessage.success('Người dùngCập nhật thành công');
    } else {
      // Thêm người dùng
      await api.post('/admin/users', {
        username: userForm.username,
        email: userForm.email,
        password: userForm.password,
        role: userForm.role,
      });
      ElMessage.success('Người dùngThêm thành công');
    }

    userDialogVisible.value = false;
    loadUserList();
  } catch (error) {
    ElMessage.error(isEditMode.value ? 'Cập nhật người dùng thất bại' : 'Thêm người dùngthất bại');
  } finally {
    userSubmitLoading.value = false;
  }
};

// XóaNgười dùng
const handleDeleteUser = async (user) => {
  try {
    await ElMessageBox.confirm(`Xác nhậnmuốnXóaNgười dùng "${user.username}"  không？`, 'XóaXác nhận', {
      confirmButtonText: 'Xác nhận',
      cancelButtonText: 'Hủy',
      type: 'warning',
    });

    await api.delete(`/admin/users/${user.id}`);
    ElMessage.success('Người dùngXóathành công');
    loadUserList();
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('XóaNgười dùngthất bại');
    }
  }
};

// MởĐặt lại mật khẩudialog
const openResetPasswordDialog = (user) => {
  currentUser.value = user;
  resetPasswordDialogVisible.value = true;
};

// MởHạn mức nhân bản giọngcài đặt
const openQuotaDialog = async (user) => {
  quotaUser.value = user;
  quotaDialogVisible.value = true;
  await loadQuotaSettings(user.id);
};

const loadQuotaSettings = async (userID) => {
  quotaLoading.value = true;
  try {
    const response = await api.get(`/admin/users/${userID}/voice-clone-quotas`);
    const quotas = response.data?.data?.quotas || [];
    quotaRows.value = quotas.map((item) => ({
      ...item,
      max_count: Number.isFinite(Number(item.max_count)) ? Number(item.max_count) : -1,
      used_count: Number(item.used_count || 0),
      remaining_count: Number.isFinite(Number(item.remaining_count)) ? Number(item.remaining_count) : -1,
    }));
    quotaOriginalMaxMap.value = quotaRows.value.reduce((acc, row) => {
      acc[row.tts_config_id] = Number(row.max_count);
      return acc;
    }, {});
  } catch (error) {
    ElMessage.error('TảiHạn mức nhân bản giọngthất bại');
    quotaRows.value = [];
    quotaOriginalMaxMap.value = {};
  } finally {
    quotaLoading.value = false;
  }
};

const saveQuotaSettings = async () => {
  if (!quotaUser.value?.id) return;
  const normalizedItems = quotaRows.value.map((row) => ({
    tts_config_id: row.tts_config_id,
    max_count: Number(row.max_count),
  }));
  for (const item of normalizedItems) {
    if (!item.tts_config_id) {
      ElMessage.error('Có tts_config_id không hợp lệ');
      return;
    }
    if (!Number.isInteger(item.max_count) || item.max_count < -1) {
      ElMessage.error('max_count chỉ được là số nguyên lớn hơn hoặc bằng -1');
      return;
    }
  }

  const items = normalizedItems.filter((item) => quotaOriginalMaxMap.value[item.tts_config_id] !== item.max_count);
  if (items.length === 0) {
    ElMessage.info('Hạn mức chưa thay đổi');
    return;
  }

  quotaSaving.value = true;
  try {
    await api.put(`/admin/users/${quotaUser.value.id}/voice-clone-quotas`, { items });
    ElMessage.success('Hạn mức nhân bản giọngLưuthành công');
    await loadQuotaSettings(quotaUser.value.id);
  } catch (error) {
    ElMessage.error('LưuHạn mức nhân bản giọngthất bại');
  } finally {
    quotaSaving.value = false;
  }
};

const resetQuotaDialog = () => {
  quotaRows.value = [];
  quotaUser.value = {};
  quotaOriginalMaxMap.value = {};
};

// Đặt lại mật khẩuform
const resetPasswordForm = () => {
  passwordForm.newPassword = '';
  passwordForm.confirmPassword = '';
  if (passwordFormRef.value) {
    passwordFormRef.value.resetFields();
  }
};

// Xử lýĐặt lại mật khẩu
const handleResetPassword = async () => {
  if (!passwordFormRef.value) return;

  try {
    await passwordFormRef.value.validate();

    await ElMessageBox.confirm(
      `Xác nhậnmuốnđặt lạiNgười dùng "${currentUser.value.username}" Mật khẩu không？`,
      'Đặt lại mật khẩuXác nhận',
      {
        confirmButtonText: 'Xác nhận',
        cancelButtonText: 'Hủy',
        type: 'warning',
      },
    );

    resetPasswordLoading.value = true;

    await api.post(`/admin/users/${currentUser.value.id}/reset-password`, {
      new_password: passwordForm.newPassword,
    });

    ElMessage.success('Mật khẩuđặt lạithành công');
    resetPasswordDialogVisible.value = false;
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Đặt lại mật khẩuthất bại');
    }
  } finally {
    resetPasswordLoading.value = false;
  }
};

// Định dạng ngày giờ
const formatDateTime = (dateString) => {
  if (!dateString) return '--';
  return new Date(dateString).toLocaleString('zh-CN');
};

// Tải dữ liệu khi component được mount
onMounted(() => {
  loadUserList();
});
</script>

<style scoped>
.config-page {
  padding: 20px;
  background: rgba(255, 255, 255, 0.88);
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.page-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.quota-hint {
  color: #666;
  font-size: 13px;
}
</style>
