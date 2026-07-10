<template>
  <div class="config-page">
    <div class="page-actions">
      <el-button
        type="warning"
        plain
        :loading="testingAll"
        @click="testAllConfigs"
        :disabled="!getEnabledConfigs().length"
      >
        Kiểm tra tất cả
      </el-button>
      <el-button type="primary" @click="showDialog = true">
        <el-icon><Plus /></el-icon>
        Thêm cấu hình
      </el-button>
    </div>

    <el-table :data="configs" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="Tên cấu hình" />
      <el-table-column prop="config_id" label="ID cấu hình" width="150" />
      <el-table-column prop="provider" label="Nhà cung cấp">
        <template #default="scope">
          {{ scope.row.provider }}
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="Trạng thái kích hoạt" width="80" align="center">
        <template #default="scope">
          <el-switch v-model="scope.row.enabled" @change="toggleEnable(scope.row)" />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="Cấu hình mặc định" width="80" align="center">
        <template #default="scope">
          <el-switch
            v-model="scope.row.is_default"
            @change="toggleDefault(scope.row)"
            :disabled="scope.row.is_default && getEnabledConfigs().length === 1"
          />
        </template>
      </el-table-column>
      <el-table-column label="Kết quả kiểm tra" width="120" align="center">
        <template #default="scope">
          <template v-if="testResults[scope.row.config_id]">
            <el-tooltip
              v-if="testResults[scope.row.config_id].ok"
              :content="formatTestResultTip(testResults[scope.row.config_id])"
              placement="top"
            >
              <span class="test-result test-ok">{{ formatTestResultLabel(testResults[scope.row.config_id]) }}</span>
            </el-tooltip>
            <el-tooltip v-else :content="testResults[scope.row.config_id].message" placement="top" :show-after="200">
              <span class="test-result test-err">Lỗi</span>
            </el-tooltip>
          </template>
          <span v-else class="test-result test-none">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="Thời gian tạo" width="180">
        <template #default="scope">
          {{ formatDate(scope.row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="Thao tác" width="260">
        <template #default="scope">
          <el-button size="small" @click="editConfig(scope.row)">Chỉnh sửa</el-button>
          <el-button
            size="small"
            type="warning"
            :loading="testingId === scope.row.config_id"
            @click="testConfig(scope.row, 'vad')"
          >
            Kiểm tra
          </el-button>
          <el-button size="small" type="danger" @click="deleteConfig(scope.row.id)"> Xóa </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Thêm/Chỉnh sửa Dialog cấu hình -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Chỉnh sửa cấu hình VAD' : 'Thêm cấu hình VAD'"
      width="720px"
      @close="handleDialogClose"
    >
      <VADConfigForm ref="formRef" :model="form" :rules="rules" />

      <template #footer>
        <el-button @click="handleDialogClose">Hủy</el-button>
        <el-button type="warning" plain @click="testCurrentConfig" :loading="testingCurrent"> Kiểm tra </el-button>
        <el-button type="primary" @click="handleSave" :loading="saving"> Lưu </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import api from '../../utils/api';
import { testSingleConfig, testWithData, parseJsonData } from '../../utils/configTest';
import VADConfigForm from './forms/VADConfigForm.vue';
import { resolveVADProvider } from './forms/configProviderUtils';

const configs = ref([]);
const testingId = ref(null);
const testingAll = ref(false);
const testingCurrent = ref(false);
const testResults = ref({});
const loading = ref(false);
const saving = ref(false);
const showDialog = ref(false);
const editingConfig = ref(null);
const formRef = ref();

const form = reactive({
  name: '',
  config_id: '',
  provider: 'ten_vad',
  is_default: false,
  enabled: true,
  webrtc_vad: {
    pool_min_size: 5,
    pool_max_size: 1000,
    pool_max_idle: 100,
    vad_sample_rate: 16000,
    vad_mode: 2,
  },
  silero_vad: {
    model_path: 'config/models/vad/silero_vad.onnx',
    threshold: 0.5,
    min_silence_duration_ms: 100,
    sample_rate: 16000,
    channels: 1,
    acquire_timeout_ms: 3000,
  },
  ten_vad: {
    hop_size: 320,
    threshold: 0.3,
    pool_size: 10,
    acquire_timeout_ms: 3000,
  },
});

const rules = {
  name: [{ required: true, message: 'Vui lòng nhậpTên cấu hình', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Vui lòng nhậpID cấu hình', trigger: 'blur' }],
  provider: [{ required: true, message: 'Vui lòng chọnNhà cung cấp', trigger: 'change' }],
  'webrtc_vad.pool_min_size': [{ required: true, message: 'Vui lòng nhập kích thước pool tối thiểu', trigger: 'blur' }],
  'webrtc_vad.pool_max_size': [{ required: true, message: 'Vui lòng nhập kích thước pool tối đa', trigger: 'blur' }],
  'webrtc_vad.pool_max_idle': [{ required: true, message: 'Vui lòng nhập số kết nối rảnh tối đa', trigger: 'blur' }],
  'webrtc_vad.vad_sample_rate': [{ required: true, message: 'Vui lòng chọn tần số lấy mẫu VAD', trigger: 'change' }],
  'webrtc_vad.vad_mode': [{ required: true, message: 'Vui lòng chọn chế độ VAD', trigger: 'change' }],
  'silero_vad.model_path': [{ required: true, message: 'Vui lòng nhập đường dẫn mô hình', trigger: 'blur' }],
  'silero_vad.threshold': [{ required: true, message: 'Vui lòng nhập ngưỡng', trigger: 'blur' }],
  'silero_vad.min_silence_duration_ms': [
    { required: true, message: 'Vui lòng nhập thời gian im lặng tối thiểu', trigger: 'blur' },
  ],
  'silero_vad.sample_rate': [{ required: true, message: 'Vui lòng chọn tần số lấy mẫu', trigger: 'change' }],
  'silero_vad.channels': [{ required: true, message: 'Vui lòng chọn số kênh âm thanh', trigger: 'change' }],
  'silero_vad.acquire_timeout_ms': [{ required: true, message: 'Vui lòng nhập thời gian chờ tối đa', trigger: 'blur' }],
  'ten_vad.hop_size': [{ required: true, message: 'Vui lòng nhập kích thước hop size', trigger: 'blur' }],
  'ten_vad.threshold': [{ required: true, message: 'Vui lòng nhập ngưỡng phát hiện VAD', trigger: 'blur' }],
  'ten_vad.pool_size': [{ required: true, message: 'Vui lòng nhập kích thước connection pool', trigger: 'blur' }],
  'ten_vad.acquire_timeout_ms': [{ required: true, message: 'Vui lòng nhập thời gian chờ tối đa', trigger: 'blur' }],
};

const loadConfigs = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/vad-configs');
    configs.value = (response.data.data || []).map(normalizeVADConfigRow);
  } catch (error) {
    ElMessage.error('Tải cấu hình thất bại');
  } finally {
    loading.value = false;
  }
};

function normalizeVADConfigRow(row) {
  const data = parseJsonData(row?.json_data);
  return {
    ...row,
    provider: resolveVADProvider(row?.provider, row?.config_id, data),
  };
}

const editConfig = (config) => {
  config = normalizeVADConfigRow(config);
  editingConfig.value = config;
  form.name = config.name;
  form.config_id = config.config_id;
  form.provider = config.provider;
  form.is_default = config.is_default;
  form.enabled = config.enabled;

  // Phân tích JSON cấu hình và điền vào các trường tương ứng
  try {
    const configObj = JSON.parse(config.json_data || '{}');
    if (configObj.webrtc_vad) {
      form.webrtc_vad = { ...form.webrtc_vad, ...configObj.webrtc_vad };
    } else if (configObj.silero_vad) {
      form.silero_vad = { ...form.silero_vad, ...configObj.silero_vad };
    } else if (configObj.ten_vad) {
      form.ten_vad = { ...form.ten_vad, ...configObj.ten_vad };
    } else {
      if (config.provider === 'webrtc_vad') {
        form.webrtc_vad = { ...form.webrtc_vad, ...configObj };
      } else if (config.provider === 'silero_vad') {
        form.silero_vad = { ...form.silero_vad, ...configObj };
      } else if (config.provider === 'ten_vad') {
        form.ten_vad = { ...form.ten_vad, ...configObj };
      }
    }
  } catch (error) {
    console.error('Lỗi phân tích JSON cấu hình:', error);
  }

  showDialog.value = true;
};

const handleSave = async () => {
  if (!formRef.value) return;

  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true;
      try {
        // Nếu là cấu hình mới và chưa có cấu hình nào，thì tự động đặt làmCấu hình mặc định
        const isFirstConfig = !editingConfig.value && configs.value.length === 0;

        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default, // Tự động đặt làm mặc định khi thêm lần đầu
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: formRef.value.getJsonData(),
        };

        if (editingConfig.value) {
          await api.put(`/admin/vad-configs/${editingConfig.value.id}`, configData);
          ElMessage.success('Cập nhật cấu hình thành công');
        } else {
          await api.post('/admin/vad-configs', configData);
          ElMessage.success('Tạo cấu hình thành công');
        }

        showDialog.value = false;
        loadConfigs();
      } catch (error) {
        ElMessage.error('Lưuthất bại: ' + (error.response?.data?.message || error.message));
      } finally {
        saving.value = false;
      }
    }
  });
};

const toggleEnable = async (config) => {
  try {
    await api.post(`/admin/configs/${config.id}/toggle`);
    ElMessage.success(`${config.enabled ? 'Bật' : 'Tắt'}thành công`);
  } catch (error) {
    // Khôi phục trạng thái nút chuyển
    config.enabled = !config.enabled;
    ElMessage.error('Thao tácthất bại');
  }
};

const toggleDefault = async (config) => {
  try {
    if (!config.enabled) {
      ElMessage.warning('Vui lòng bật cấu hình này trước khi đặt làm mặc định');
      config.is_default = false;
      return;
    }

    const configData = {
      name: config.name,
      config_id: config.config_id,
      provider: config.provider,
      is_default: config.is_default,
      enabled: config.enabled,
      json_data: config.json_data,
    };

    await api.put(`/admin/vad-configs/${config.id}`, configData);
    ElMessage.success(config.is_default ? 'Đặt làm mặc định thành công' : 'Hủymặc định thành công');

    // Làm mới danh sách để cập nhật trạng thái mặc định của các cấu hình khác
    loadConfigs();
  } catch (error) {
    // Khôi phục trạng thái nút chuyển
    config.is_default = !config.is_default;
    ElMessage.error('Thao tácthất bại');
  }
};

const getEnabledConfigs = () => {
  return configs.value.filter((config) => config.enabled);
};

function formatTestResultLabel(r) {
  if (!r?.ok) return 'Lỗi';
  return r.first_packet_ms != null ? `Thành công ${r.first_packet_ms}ms` : 'Thành công';
}
function formatTestResultTip(r) {
  if (!r?.ok) return '';
  return r.first_packet_ms != null ? `Thành công, thời gian phản hồi ${r.first_packet_ms}ms` : 'Thành công';
}
function formatTestMessage(result) {
  const base = result.message || '';
  return result.first_packet_ms != null ? `${base} ${result.first_packet_ms}ms` : base;
}

const testConfig = async (row, type) => {
  testingId.value = row.config_id;
  try {
    const result = await testSingleConfig(type, row.config_id);
    testResults.value = { ...testResults.value, [row.config_id]: result };
    if (result.ok) {
      ElMessage.success(`${row.name || row.config_id}：${formatTestMessage(result)}`);
    } else {
      ElMessage.warning(`${row.name || row.config_id}：${result.message}`);
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Yêu cầu kiểm tra thất bại');
  } finally {
    testingId.value = null;
  }
};

const testAllConfigs = async () => {
  const list = getEnabledConfigs();
  if (!list.length) {
    ElMessage.warning('Không có cấu hình nào được bật');
    return;
  }
  testingAll.value = true;
  testResults.value = {};
  let okCount = 0;
  try {
    for (const row of list) {
      try {
        const result = await testSingleConfig('vad', row.config_id);
        testResults.value = { ...testResults.value, [row.config_id]: result };
        if (result.ok) okCount++;
      } catch (_) {
        testResults.value = { ...testResults.value, [row.config_id]: { ok: false, message: 'Yêu cầu thất bại' } };
      }
    }
    ElMessage.success(`Hoàn thành kiểm tra tất cả: ${okCount}/${list.length}`);
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Yêu cầu kiểm tra thất bại');
  } finally {
    testingAll.value = false;
  }
};

const testCurrentConfig = async () => {
  if (!formRef.value) return;
  try {
    await formRef.value.validate();
  } catch (_) {
    return;
  }
  const configId = form.config_id?.trim();
  if (!configId) {
    ElMessage.warning('Vui lòng điềnID cấu hình');
    return;
  }
  const payload = {
    name: form.name,
    config_id: configId,
    provider: form.provider,
    is_default: form.is_default,
    ...parseJsonData(formRef.value.getJsonData()),
  };
  testingCurrent.value = true;
  try {
    const result = await testWithData('vad', { [configId]: payload });
    if (result.ok) {
      ElMessage.success(formatTestMessage(result) || 'Kiểm tra thành công');
    } else {
      ElMessage.warning(result.message || 'Kiểm tra không thành công');
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Yêu cầu kiểm tra thất bại');
  } finally {
    testingCurrent.value = false;
  }
};

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Xác nhậnmuốnXóacấu hình này không？', 'Thông báo', {
      confirmButtonText: 'Xác nhận',
      cancelButtonText: 'Hủy',
      type: 'warning',
    });

    await api.delete(`/admin/vad-configs/${id}`);
    ElMessage.success('Xóathành công');
    loadConfigs();
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Xóathất bại');
    }
  }
};

const resetForm = () => {
  editingConfig.value = null;
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'ten_vad',
    is_default: false,
    enabled: true,
    webrtc_vad: {
      pool_min_size: 5,
      pool_max_size: 1000,
      pool_max_idle: 100,
      vad_sample_rate: 16000,
      vad_mode: 2,
    },
    silero_vad: {
      model_path: 'config/models/vad/silero_vad.onnx',
      threshold: 0.5,
      min_silence_duration_ms: 100,
      sample_rate: 16000,
      channels: 1,
      acquire_timeout_ms: 3000,
    },
    ten_vad: {
      hop_size: 320,
      threshold: 0.3,
      pool_size: 10,
      acquire_timeout_ms: 3000,
    },
  });
};

const handleDialogClose = () => {
  showDialog.value = false;
  resetForm();
  if (formRef.value) {
    formRef.value.resetFields();
  }
};

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN');
};

onMounted(() => {
  loadConfigs();
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
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.test-result {
  font-size: 12px;
}
.test-result.test-ok {
  color: var(--el-color-success);
}
.test-result.test-err {
  color: var(--el-color-danger);
  cursor: help;
}
.test-result.test-none {
  color: var(--el-text-color-placeholder);
}
</style>
