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
            @click="testConfig(scope.row, 'asr')"
          >
            Kiểm tra
          </el-button>
          <el-button size="small" type="danger" @click="deleteConfig(scope.row.id)"> Xóa </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Dialog thêm/chỉnh sửa cấu hình -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Chỉnh sửa cấu hình ASR' : 'Thêm cấu hình ASR'"
      width="720px"
      @close="handleDialogClose"
    >
      <ASRConfigForm ref="formRef" :model="form" :rules="rules" />

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
import ASRConfigForm from './forms/ASRConfigForm.vue';
import { resolveASRProvider } from './forms/configProviderUtils';

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

const validateAliyunPcm = (rule, value, callback) => {
  if (value !== 'pcm') {
    callback(new Error('Định dạng phải là pcm'));
    return;
  }
  callback();
};

const validateAliyun16000 = (rule, value, callback) => {
  if (Number(value) !== 16000) {
    callback(new Error('Tần số lấy mẫu phải là 16000'));
    return;
  }
  callback();
};

const form = reactive({
  name: '',
  config_id: '',
  provider: '',
  is_default: false,
  enabled: true,
  funasr: {
    host: 'localhost',
    port: 10095,
    mode: 'offline',
    sample_rate: 16000,
    chunk_size: [5, 10, 5],
    chunk_interval: 10,
    max_connections: 100,
    timeout: 30,
    auto_end: false,
  },
  aliyun_funasr: {
    api_key: '',
    ws_url: 'wss://dashscope-intl.aliyuncs.com/api-ws/v1/inference/',
    model: 'fun-asr-realtime',
    format: 'pcm',
    sample_rate: 16000,
    language_hints: ['zh'],
    vocabulary_id: '',
    disfluency_removal_enabled: false,
    timeout: 30,
  },
  doubao: {
    appid: '',
    access_token: '',
    ws_url: 'wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async',
    model_name: 'bigmodel',
    end_window_size: 800,
    enable_punc: true,
    enable_itn: true,
    enable_ddc: false,
    chunk_duration: 200,
    timeout: 30,
  },
  aliyun_qwen3: {
    api_key: '',
    ws_url: 'wss://dashscope-intl.aliyuncs.com/api-ws/v1/realtime',
    model: 'qwen3-asr-flash-realtime',
    format: 'pcm',
    sample_rate: 16000,
    language: 'vi',
    auto_end: false,
    vad_threshold: 0.0,
    vad_silence_ms: 400,
    timeout: 30,
  },
  xunfei: {
    appid: '',
    api_key: '',
    api_secret: '',
    host: 'iat-api.xfyun.cn',
    path: '/v2/iat',
    domain: 'iat',
    language: 'zh_cn',
    accent: 'mandarin',
    sample_rate: 16000,
    timeout: 30,
  },
});

// Quy tắc động theo provider hiện tại, tránh các trường doubao/funasr ẩn gây lỗi bắt buộc khi lưu
const rules = computed(() => {
  const base = {
    name: [{ required: true, message: 'Vui lòng nhập tên cấu hình', trigger: 'blur' }],
    config_id: [{ required: true, message: 'Vui lòng nhập ID cấu hình', trigger: 'blur' }],
    provider: [{ required: true, message: 'Vui lòng chọn nhà cung cấp', trigger: 'change' }],
  };
  if (form.provider === 'funasr') {
    return {
      ...base,
      'funasr.host': [{ required: true, message: 'Vui lòng nhập địa chỉ máy chủ', trigger: 'blur' }],
      'funasr.port': [{ required: true, message: 'Vui lòng nhập cổng', trigger: 'blur' }],
      'funasr.mode': [{ required: true, message: 'Vui lòng chọn chế độ', trigger: 'change' }],
      'funasr.sample_rate': [{ required: true, message: 'Vui lòng chọn tần số lấy mẫu', trigger: 'change' }],
      'funasr.chunk_size': [{ required: true, message: 'Vui lòng nhập kích thước khối', trigger: 'blur' }],
      'funasr.chunk_interval': [{ required: true, message: 'Vui lòng nhập khoảng cách khối', trigger: 'blur' }],
      'funasr.max_connections': [{ required: true, message: 'Vui lòng nhập số kết nối tối đa', trigger: 'blur' }],
      'funasr.timeout': [{ required: true, message: 'Vui lòng nhập thời gian chờ', trigger: 'blur' }],
    };
  }
  if (form.provider === 'aliyun_funasr') {
    return {
      ...base,
      'aliyun_funasr.ws_url': [{ required: true, message: 'Vui lòng nhập WS URL', trigger: 'blur' }],
      'aliyun_funasr.model': [{ required: true, message: 'Vui lòng nhập tên mô hình', trigger: 'blur' }],
      'aliyun_funasr.format': [
        { required: true, message: 'Vui lòng chọn định dạng âm thanh', trigger: 'change' },
        { validator: validateAliyunPcm, trigger: 'change' },
      ],
      'aliyun_funasr.sample_rate': [
        { required: true, message: 'Vui lòng chọn tần số lấy mẫu', trigger: 'change' },
        { validator: validateAliyun16000, trigger: 'change' },
      ],
      'aliyun_funasr.timeout': [{ required: true, message: 'Vui lòng nhập thời gian chờ', trigger: 'blur' }],
    };
  }
  if (form.provider === 'doubao') {
    return {
      ...base,
      'doubao.appid': [{ required: true, message: 'Vui lòng nhập ID ứng dụng', trigger: 'blur' }],
      'doubao.access_token': [{ required: true, message: 'Vui lòng nhập mã truy cập', trigger: 'blur' }],
      'doubao.ws_url': [{ required: true, message: 'Vui lòng nhập WebSocket URL', trigger: 'blur' }],
      'doubao.resource_id': [{ required: true, message: 'Vui lòng chọn thông số tài nguyên', trigger: 'change' }],
      'doubao.end_window_size': [
        { required: true, message: 'Vui lòng nhập kích thước cửa sổ kết thúc', trigger: 'blur' },
      ],
      'doubao.timeout': [{ required: true, message: 'Vui lòng nhập thời gian chờ', trigger: 'blur' }],
    };
  }
  if (form.provider === 'aliyun_qwen3') {
    return {
      ...base,
      'aliyun_qwen3.ws_url': [{ required: true, message: 'Vui lòng nhập WS URL', trigger: 'blur' }],
      'aliyun_qwen3.model': [{ required: true, message: 'Vui lòng nhập tên mô hình', trigger: 'blur' }],
      'aliyun_qwen3.format': [{ required: true, message: 'Vui lòng chọn định dạng âm thanh', trigger: 'change' }],
      'aliyun_qwen3.sample_rate': [{ required: true, message: 'Vui lòng chọn tần số lấy mẫu', trigger: 'change' }],
      'aliyun_qwen3.language': [{ required: true, message: 'Vui lòng nhập ngôn ngữ', trigger: 'blur' }],
      'aliyun_qwen3.timeout': [{ required: true, message: 'Vui lòng nhập thời gian chờ', trigger: 'blur' }],
    };
  }
  if (form.provider === 'xunfei') {
    return {
      ...base,
      'xunfei.appid': [{ required: true, message: 'Vui lòng nhập ID ứng dụng', trigger: 'blur' }],
      'xunfei.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
      'xunfei.api_secret': [{ required: true, message: 'Vui lòng nhập API Secret', trigger: 'blur' }],
      'xunfei.host': [{ required: true, message: 'Vui lòng nhập Host', trigger: 'blur' }],
      'xunfei.path': [{ required: true, message: 'Vui lòng nhập Path', trigger: 'blur' }],
      'xunfei.sample_rate': [{ required: true, message: 'Vui lòng nhập tần số lấy mẫu', trigger: 'change' }],
      'xunfei.timeout': [{ required: true, message: 'Vui lòng nhập thời gian chờ', trigger: 'blur' }],
    };
  }
  return base;
});

const loadConfigs = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/asr-configs');
    configs.value = (response.data.data || []).map(normalizeASRConfigRow);
  } catch (error) {
    ElMessage.error('Tải cấu hình thất bại');
  } finally {
    loading.value = false;
  }
};

function normalizeASRConfigRow(row) {
  const data = parseJsonData(row?.json_data);
  return {
    ...row,
    provider: resolveASRProvider(row?.provider, row?.config_id, data),
  };
}

const editConfig = (config) => {
  config = normalizeASRConfigRow(config);
  editingConfig.value = config;
  form.name = config.name;
  form.config_id = config.config_id;
  form.provider = config.provider;
  form.is_default = config.is_default;
  form.enabled = config.enabled;

  // Phân tích JSON cấu hình và điền vào các trường tương ứng
  try {
    const configObj = JSON.parse(config.json_data || '{}');

    // Tương thích định dạng cũ/mới: kiểm tra có phải dạng wrapper (chứa lớp provider) hay dạng trực tiếp
    if (configObj.funasr) {
      // Định dạng cũ: chứa lớp provider
      const funasrConfig = { ...form.funasr, ...configObj.funasr };
      // Tương thích chunk_size: nếu là số đơn hoặc định dạng không hợp lệ, chuyển về mặc định [5, 10, 5]
      if (typeof funasrConfig.chunk_size === 'number') {
        funasrConfig.chunk_size = [5, 10, 5];
      } else if (!Array.isArray(funasrConfig.chunk_size) || funasrConfig.chunk_size.length !== 3) {
        funasrConfig.chunk_size = [5, 10, 5];
      }
      form.funasr = funasrConfig;
    } else if (configObj.aliyun_funasr) {
      // Định dạng cũ: chứa lớp provider
      form.aliyun_funasr = { ...form.aliyun_funasr, ...configObj.aliyun_funasr };
    } else if (configObj.doubao) {
      // Định dạng cũ: chứa lớp provider
      form.doubao = { ...form.doubao, ...configObj.doubao };
    } else if (config.provider === 'funasr' && configObj.host) {
      // Định dạng mới: chứa trực tiếp nội dung cấu hình
      const funasrConfig = { ...form.funasr, ...configObj };
      // Tương thích chunk_size: nếu là số đơn hoặc định dạng không hợp lệ, chuyển về mặc định [5, 10, 5]
      if (typeof funasrConfig.chunk_size === 'number') {
        funasrConfig.chunk_size = [5, 10, 5];
      } else if (!Array.isArray(funasrConfig.chunk_size) || funasrConfig.chunk_size.length !== 3) {
        funasrConfig.chunk_size = [5, 10, 5];
      }
      form.funasr = funasrConfig;
    } else if (config.provider === 'aliyun_funasr' && (configObj.ws_url || configObj.model || configObj.api_key)) {
      // Định dạng mới: chứa trực tiếp nội dung cấu hình
      form.aliyun_funasr = { ...form.aliyun_funasr, ...configObj };
    } else if (config.provider === 'doubao' && (configObj.appid || configObj.access_token)) {
      // Định dạng mới: chứa trực tiếp nội dung cấu hình
      form.doubao = { ...form.doubao, ...configObj };
    } else if (configObj.aliyun_qwen3) {
      // Định dạng cũ: chứa lớp provider
      form.aliyun_qwen3 = { ...form.aliyun_qwen3, ...configObj.aliyun_qwen3 };
    } else if (config.provider === 'aliyun_qwen3' && (configObj.ws_url || configObj.model || configObj.api_key)) {
      // Định dạng mới: chứa trực tiếp nội dung cấu hình
      form.aliyun_qwen3 = { ...form.aliyun_qwen3, ...configObj };
    } else if (configObj.xunfei) {
      form.xunfei = { ...form.xunfei, ...configObj.xunfei };
    } else if (config.provider === 'xunfei' && (configObj.appid || configObj.api_key || configObj.api_secret)) {
      form.xunfei = { ...form.xunfei, ...configObj };
    }
  } catch (error) {
    console.error('Phân tích JSON cấu hình thất bại:', error);
  }

  showDialog.value = true;
};

const handleSave = async () => {
  if (!formRef.value) {
    ElMessage.warning('Form chưa sẵn sàng, vui lòng thử lại sau');
    return;
  }
  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true;
      try {
        // Nếu là cấu hình đầu tiên, tự động đặt làm mặc định
        const isFirstConfig = !editingConfig.value && configs.value.length === 0;

        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default, // Tự động đặt mặc định khi thêm lần đầu
          enabled: form.enabled !== undefined ? form.enabled : true, // Đảm bảo trường enabled tồn tại
          json_data: formRef.value.getJsonData(),
        };

        if (editingConfig.value) {
          await api.put(`/admin/asr-configs/${editingConfig.value.id}`, configData);
          ElMessage.success('Cập nhật cấu hình thành công');
        } else {
          await api.post('/admin/asr-configs', configData);
          ElMessage.success('Tạo cấu hình thành công');
        }

        showDialog.value = false;
        loadConfigs();
      } catch (error) {
        const msg = error.response?.data?.error || error.response?.data?.message || error.message;
        ElMessage.error('Lưu thất bại: ' + msg);
      } finally {
        saving.value = false;
      }
    }
  });
};

const toggleEnable = async (config) => {
  try {
    await api.post(`/admin/configs/${config.id}/toggle`);
    ElMessage.success(`${config.enabled ? 'Kích hoạt' : 'Tắt'} thành công`);
  } catch (error) {
    // Khôi phục trạng thái công tắc
    config.enabled = !config.enabled;
    ElMessage.error('Thao tác thất bại');
  }
};

const toggleDefault = async (config) => {
  try {
    if (!config.enabled) {
      ElMessage.warning('Vui lòng kích hoạt cấu hình này trước khi đặt làm mặc định');
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

    await api.put(`/admin/asr-configs/${config.id}`, configData);
    ElMessage.success(config.is_default ? 'Đặt làm mặc định thành công' : 'Bỏ mặc định thành công');

    // Làm mới danh sách để cập nhật trạng thái mặc định của các cấu hình khác
    loadConfigs();
  } catch (error) {
    // Khôi phục trạng thái công tắc
    config.is_default = !config.is_default;
    ElMessage.error('Thao tác thất bại');
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
      ElMessage.success(`${row.name || row.config_id}: ${formatTestMessage(result)}`);
    } else {
      ElMessage.warning(`${row.name || row.config_id}: ${result.message}`);
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
    ElMessage.warning('Không có cấu hình nào được kích hoạt');
    return;
  }
  testingAll.value = true;
  testResults.value = {};
  let okCount = 0;
  try {
    for (const row of list) {
      try {
        const result = await testSingleConfig('asr', row.config_id);
        testResults.value = { ...testResults.value, [row.config_id]: result };
        if (result.ok) okCount++;
      } catch (_) {
        testResults.value = { ...testResults.value, [row.config_id]: { ok: false, message: 'Yêu cầu thất bại' } };
      }
    }
    ElMessage.success(`Hoàn tất kiểm tra: ${okCount}/${list.length} thành công`);
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
    ElMessage.warning('Vui lòng điền ID cấu hình');
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
    const result = await testWithData('asr', { [configId]: payload });
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
    await ElMessageBox.confirm('Bạn có chắc muốn xóa cấu hình này không?', 'Xác nhận', {
      confirmButtonText: 'Xác nhận',
      cancelButtonText: 'Hủy',
      type: 'warning',
    });

    await api.delete(`/admin/asr-configs/${id}`);
    ElMessage.success('Xóa thành công');
    loadConfigs();
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Xóa thất bại');
    }
  }
};

const resetForm = () => {
  editingConfig.value = null;
  form.name = '';
  form.config_id = '';
  form.provider = '';
  form.is_default = false;
  form.enabled = true;
  form.funasr = {
    host: 'localhost',
    port: 10095,
    mode: 'offline',
    sample_rate: 16000,
    chunk_size: [5, 10, 5],
    chunk_interval: 10,
    max_connections: 100,
    timeout: 30,
    auto_end: false,
  };
  form.aliyun_funasr = {
    api_key: '',
    ws_url: 'wss://dashscope-intl.aliyuncs.com/api-ws/v1/inference/',
    model: 'fun-asr-realtime',
    format: 'pcm',
    sample_rate: 16000,
    language_hints: ['zh'],
    vocabulary_id: '',
    disfluency_removal_enabled: false,
    timeout: 30,
  };
  form.doubao = {
    appid: '',
    access_token: '',
    ws_url: 'wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async',
    resource_id: 'volc.bigasr.sauc.duration',
    model_name: 'bigmodel',
    end_window_size: 800,
    enable_punc: true,
    enable_itn: true,
    enable_ddc: false,
    chunk_duration: 200,
    timeout: 30,
  };
  form.aliyun_qwen3 = {
    api_key: '',
    ws_url: 'wss://dashscope-intl.aliyuncs.com/api-ws/v1/realtime',
    model: 'qwen3-asr-flash-realtime',
    format: 'pcm',
    sample_rate: 16000,
    language: 'vi',
    auto_end: false,
    vad_threshold: 0.0,
    vad_silence_ms: 400,
    timeout: 30,
  };
  form.xunfei = {
    appid: '',
    api_key: '',
    api_secret: '',
    host: 'iat-api.xfyun.cn',
    path: '/v2/iat',
    domain: 'iat',
    language: 'zh_cn',
    accent: 'mandarin',
    sample_rate: 16000,
    timeout: 30,
  };
};

const handleDialogClose = () => {
  showDialog.value = false;
  resetForm();
  if (formRef.value) {
    formRef.value.resetFields();
  }
};

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('vi-VN');
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
