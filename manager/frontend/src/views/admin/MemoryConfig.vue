<template>
  <div class="config-page">
    <div class="page-actions">
      <el-button type="primary" @click="handleAddConfig">
        <el-icon><Plus /></el-icon>
        Thêm cấu hình
      </el-button>
    </div>

    <el-table :data="safeConfigs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Tên cấu hình" />
      <el-table-column prop="config_id" label="ID cấu hình" width="150" />
      <el-table-column prop="provider" label="Nhà cung cấp" width="120">
        <template #default="scope">
          <el-tag :type="getProviderTagType(scope.row.provider)">
            {{ scope.row.provider }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="Trạng thái kích hoạt" width="80" align="center">
        <template #default="scope">
          <el-switch v-model="scope.row.enabled" @change="toggleEnable(scope.row)" />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="Cấu hình mặc định" width="80" align="center">
        <template #default="scope">
          <el-switch v-model="scope.row.is_default" @change="toggleDefault(scope.row)" />
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="Thời gian tạo" width="180">
        <template #default="scope">
          {{ formatDate(scope.row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="Thao tác" width="180">
        <template #default="scope">
          <el-button size="small" @click="editConfig(scope.row)">Chỉnh sửa</el-button>
          <el-button size="small" type="danger" @click="deleteConfig(scope.row.id)"> Xóa </el-button>
        </template>
      </el-table-column>

      <!-- Slot trạng thái rỗng -->
      <template #empty>
        <div class="empty-state">
          <el-icon size="64" color="#C0C4CC" class="empty-icon">
            <Box />
          </el-icon>
          <div class="empty-text">Chưa có cấu hình Memory</div>
          <div class="empty-description">Nhấn nút "Thêm cấu hình" ở trên để tạo cấu hình Memory đầu tiên của bạn</div>
          <el-button type="primary" @click="handleAddConfig" class="empty-action">
            <el-icon><Plus /></el-icon>
            Thêm cấu hình
          </el-button>
        </div>
      </template>
    </el-table>

    <!-- Hộp thoại thêm/chỉnh sửa cấu hình -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Chỉnh sửa cấu hình Memory' : 'Thêm cấu hình Memory'"
      width="720px"
      @close="handleDialogClose"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="150px">
        <el-form-item label="Nhà cung cấp" prop="provider">
          <el-select
            v-model="form.provider"
            placeholder="Vui lòng chọn nhà cung cấp"
            style="width: 100%"
            @change="handleProviderChange"
          >
            <el-option label="Memobase" value="memobase" />
            <el-option label="Mem0" value="mem0" />
            <el-option label="MemOS" value="memos" />
          </el-select>
        </el-form-item>

        <el-form-item label="Tên cấu hình" prop="name">
          <el-input v-model="form.name" placeholder="Vui lòng nhập tên cấu hình" />
        </el-form-item>

        <el-form-item label="ID cấu hình" prop="config_id">
          <el-input v-model="form.config_id" placeholder="Vui lòng nhập ID cấu hình duy nhất" />
        </el-form-item>

        <!-- Các trường cấu hình Memobase -->
        <template v-if="form.provider === 'memobase'">
          <el-form-item label="API key" prop="api_key">
            <el-input
              v-model="form.api_key"
              type="password"
              placeholder="Vui lòng nhập API key Memobase"
              show-password
            />
          </el-form-item>

          <el-form-item label="baseURL" prop="base_url">
            <el-input v-model="form.base_url" placeholder="Vui lòng nhập baseURL Memobase" />
          </el-form-item>

          <el-form-item label="Bật tìm kiếm" prop="enable_search">
            <el-switch v-model="form.enable_search" />
          </el-form-item>

          <el-form-item label="Ngưỡng tìm kiếm" prop="search_threshold">
            <el-input-number
              v-model="form.search_threshold"
              :min="0"
              :max="1"
              :step="0.1"
              :precision="1"
              style="width: 100%"
            />
          </el-form-item>

          <el-form-item label="Số kết quả TopK" prop="search_top_k">
            <el-input-number v-model="form.search_top_k" :min="1" :step="1" style="width: 100%" />
          </el-form-item>
        </template>

        <!-- Các trường cấu hình Mem0 -->
        <template v-if="form.provider === 'mem0' || form.provider === 'memos'">
          <el-form-item label="API key" prop="api_key">
            <el-input
              v-model="form.api_key"
              type="password"
              :placeholder="
                form.provider === 'memos' ? 'Vui lòng nhập API key tương thích MemOS' : 'Vui lòng nhập API key Mem0'
              "
              show-password
            />
          </el-form-item>

          <el-form-item label="baseURL" prop="base_url">
            <el-input
              v-model="form.base_url"
              :placeholder="
                form.provider === 'memos' ? 'Vui lòng nhập baseURL dịch vụ MemOS' : 'Vui lòng nhập baseURL Mem0'
              "
            />
          </el-form-item>

          <el-form-item label="Bật tìm kiếm" prop="enable_search">
            <el-switch v-model="form.enable_search" />
          </el-form-item>

          <el-form-item label="Ngưỡng tìm kiếm" prop="search_threshold">
            <el-input-number
              v-model="form.search_threshold"
              :min="0"
              :max="1"
              :step="0.1"
              :precision="1"
              style="width: 100%"
            />
          </el-form-item>

          <el-form-item label="Số kết quả TopK" prop="search_top_k">
            <el-input-number v-model="form.search_top_k" :min="1" :step="1" style="width: 100%" />
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="handleDialogClose">Hủy</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving"> Lưu </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed, nextTick } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus, Box } from '@element-plus/icons-vue';
import api from '../../utils/api';
import { resolveMemoryProvider } from './forms/configProviderUtils';

const configs = ref([]);
const loading = ref(false);
const saving = ref(false);
const showDialog = ref(false);
const editingConfig = ref(null);
const formRef = ref();

// Đảm bảo configs luôn là một mảng
const safeConfigs = computed(() => {
  return Array.isArray(configs.value) ? configs.value : [];
});

const form = reactive({
  name: '',
  config_id: '',
  provider: 'memobase',
  is_default: false,
  enabled: true,
  api_key: '',
  base_url: '',
  enable_search: true,
  search_threshold: 0.5,
  search_top_k: 3,
  timeout_ms: 10000,
});

// Cấu hình URL mặc định
const defaultUrls = {
  memobase: 'https://api.memobase.dev',
  mem0: 'https://api.mem0.ai',
  memos: 'https://memos.memtensor.cn/api/openmem/v1',
};

const getProviderTagType = (provider) => {
  if (provider === 'memobase') return 'primary';
  if (provider === 'memos') return 'warning';
  return 'success';
};

const handleProviderChange = (value) => {
  // Xóa các trường form
  form.api_key = '';
  form.base_url = defaultUrls[value] || '';
  form.enable_search = true;
  form.search_threshold = 0.5;
  form.search_top_k = 3;
  form.timeout_ms = 10000;
};

// Tạo chuỗi JSON cấu hình
const generateConfig = () => {
  const config = {
    api_key: form.api_key,
    base_url: form.base_url,
    enable_search: form.enable_search,
    search_threshold: form.search_threshold,
    search_top_k: form.search_top_k,
  };

  if (form.provider === 'memos') {
    config.timeout_ms = form.timeout_ms;
  }

  return JSON.stringify(config);
};

// Phân tích chuỗi JSON cấu hình
const parseConfig = (jsonData) => {
  try {
    const config = JSON.parse(jsonData);
    form.api_key = config.api_key || '';
    form.base_url = config.base_url || defaultUrls[form.provider] || '';
    form.enable_search = config.enable_search !== undefined ? config.enable_search : true;
    form.search_threshold = config.search_threshold !== undefined ? config.search_threshold : 0.5;
    form.search_top_k = config.search_top_k !== undefined ? config.search_top_k : 3;
    form.timeout_ms = config.timeout_ms !== undefined ? config.timeout_ms : 10000;
  } catch (error) {
    console.error('Phân tích cấu hình thất bại:', error);
  }
};

const rules = {
  name: [{ required: true, message: 'Vui lòng nhập tên cấu hình', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Vui lòng nhập ID cấu hình', trigger: 'blur' }],
  provider: [{ required: true, message: 'Vui lòng chọn nhà cung cấp', trigger: 'change' }],
  api_key: [{ required: true, message: 'Vui lòng nhập API key', trigger: 'blur' }],
  base_url: [{ required: true, message: 'Vui lòng nhập baseURL', trigger: 'blur' }],
};

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('vi-VN');
};

const loadConfigs = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/memory-configs');
    console.log('API response:', response);

    // Dùng nextTick để đảm bảo cập nhật reactive an toàn
    await nextTick();

    // The backend returns { data: configs }, so we need to access response.data.data
    if (response && response.data && response.data.data && Array.isArray(response.data.data)) {
      // Dùng Object.freeze để tránh sửa đổi ngoài ý muốn, sau đó tạo mảng mới
      const newConfigs = response.data.data.map(normalizeMemoryConfigRow);
      configs.value = newConfigs;
    } else if (response && response.data && response.data.data) {
      // If response.data.data exists but is not an array, wrap it in an array
      configs.value = [normalizeMemoryConfigRow(response.data.data)];
    } else {
      // If no valid data, set to empty array
      configs.value = [];
    }
    console.log('Loaded configs:', configs.value);
  } catch (error) {
    console.error('Tải cấu hình thất bại:', error);
    ElMessage.error('Tải cấu hình thất bại: ' + (error.message || 'Lỗi không xác định'));
    // Ensure configs is always an array to prevent render errors
    configs.value = [];
  } finally {
    loading.value = false;
  }
};

function parseJsonData(jsonData) {
  if (!jsonData || typeof jsonData !== 'string') return {};
  try {
    return JSON.parse(jsonData) || {};
  } catch {
    return {};
  }
}

function normalizeMemoryConfigRow(row) {
  const data = parseJsonData(row?.json_data);
  return {
    ...row,
    provider: resolveMemoryProvider(row?.provider, row?.config_id, data),
  };
}

const handleSave = async () => {
  if (!formRef.value) return;

  try {
    await formRef.value.validate();
  } catch (error) {
    return;
  }

  saving.value = true;
  try {
    const configData = {
      name: form.name,
      config_id: form.config_id,
      provider: form.provider,
      enabled: form.enabled,
      is_default: form.is_default,
      json_data: generateConfig(),
    };

    if (editingConfig.value) {
      await api.put(`/admin/memory-configs/${editingConfig.value.id}`, configData);
      ElMessage.success('Cập nhật cấu hình thành công');
    } else {
      await api.post('/admin/memory-configs', configData);
      ElMessage.success('Tạo cấu hình thành công');
    }

    showDialog.value = false;
    await loadConfigs();
  } catch (error) {
    ElMessage.error('Lưu thất bại: ' + error.message);
  } finally {
    saving.value = false;
  }
};

const editConfig = (config) => {
  config = normalizeMemoryConfigRow(config);
  editingConfig.value = config;
  form.name = config.name;
  form.config_id = config.config_id;
  form.provider = config.provider;
  form.enabled = config.enabled;
  form.is_default = config.is_default;

  if (config.json_data) {
    parseConfig(config.json_data);
  }

  showDialog.value = true;
};

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Bạn có chắc muốn xóa cấu hình này không?', 'Xác nhận xóa', {
      type: 'warning',
    });

    await api.delete(`/admin/memory-configs/${id}`);
    ElMessage.success('Xóa thành công');
    await loadConfigs();
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Xóa thất bại: ' + error.message);
    }
  }
};

const toggleEnable = async (config) => {
  try {
    await api.put(`/admin/memory-configs/${config.id}`, {
      ...config,
      enabled: config.enabled,
    });
    ElMessage.success(config.enabled ? 'Đã bật' : 'Đã tắt');
  } catch (error) {
    config.enabled = !config.enabled;
    ElMessage.error('Thao tác thất bại: ' + error.message);
  }
};

const toggleDefault = async (config) => {
  try {
    if (config.is_default) {
      await api.post(`/admin/memory-configs/${config.id}/set-default`);
      ElMessage.success('Đã đặt làm cấu hình mặc định');
      await loadConfigs();
    } else {
      await api.put(`/admin/memory-configs/${config.id}`, {
        name: config.name,
        config_id: config.config_id,
        provider: config.provider,
        enabled: config.enabled,
        is_default: false,
        json_data: config.json_data || '',
      });
      ElMessage.success('Đã hủy cấu hình mặc định (không bật bộ nhớ dài hạn)');
      await loadConfigs();
    }
  } catch (error) {
    config.is_default = !config.is_default;
    ElMessage.error('Thao tác thất bại: ' + error.message);
  }
};

const handleAddConfig = () => {
  // Đặt lại form và thiết lập giá trị mặc định
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'memobase',
    is_default: false,
    enabled: true,
    api_key: '',
    base_url: defaultUrls['memobase'], // Đặt URL mặc định
    enable_search: true,
    search_threshold: 0.5,
    search_top_k: 3,
    timeout_ms: 10000,
  });

  editingConfig.value = null;
  showDialog.value = true;
};

const handleDialogClose = () => {
  showDialog.value = false;
  editingConfig.value = null;

  // Đặt lại form
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'memobase',
    is_default: false,
    enabled: true,
    api_key: '',
    base_url: '',
    enable_search: true,
    search_threshold: 0.5,
    search_top_k: 3,
    timeout_ms: 10000,
  });

  if (formRef.value) {
    formRef.value.clearValidate();
  }
};

onMounted(() => {
  loadConfigs();
});
</script>

<style scoped>
.config-page {
  padding: 20px;
}

.page-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
}

.empty-icon {
  margin-bottom: 16px;
}

.empty-text {
  font-size: 16px;
  color: #606266;
  margin-bottom: 8px;
  font-weight: 500;
}

.empty-description {
  font-size: 14px;
  color: #909399;
  margin-bottom: 24px;
  line-height: 1.5;
}

.empty-action {
  margin-top: 8px;
}
</style>
