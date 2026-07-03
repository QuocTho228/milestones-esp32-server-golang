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
      <el-button type="primary" @click="openCreateDialog">
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
          {{ getProviderLabel(scope.row.provider) }}
        </template>
      </el-table-column>
      <el-table-column label="Suy nghĩ" width="100" align="center">
        <template #default="scope">
          <el-tag
            v-if="getThinkingLabel(scope.row)"
            size="small"
            effect="plain"
            :type="testResults[scope.row.config_id]?.ok ? 'success' : 'info'"
            class="thinking-tag"
          >
            {{ getThinkingLabel(scope.row) }}
          </el-tag>
          <span v-else class="test-result test-none">-</span>
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
      <el-table-column label="Thời gian xử lý" width="100" align="center">
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
            @click="testConfig(scope.row, 'llm')"
          >
            Kiểm tra
          </el-button>
          <el-button size="small" type="danger" @click="deleteConfig(scope.row.id)"> Xóa </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Thêm/ Chỉnh sửa cấu hình popup -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Chỉnh sửa cấu hình LLM' : 'Thêm cấu hình LLM'"
      width="600px"
      @close="handleDialogClose"
    >
      <LLMConfigForm ref="formRef" :model="form" :rules="rules" />

      <template #footer>
        <el-button @click="handleDialogClose">Hủy</el-button>
        <el-button type="warning" plain @click="testCurrentConfig" :loading="testingCurrent"> Kiểm tra </el-button>
        <el-button type="primary" @click="handleSave" :loading="saving"> Lưu </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, nextTick } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import api from '../../utils/api';
import { testSingleConfig, testWithData, parseJsonData } from '../../utils/configTest';
import LLMConfigForm from './forms/LLMConfigForm.vue';
import {
  getProviderFixedType,
  getProviderThinkingConfig,
  isProviderBaseURLEditable,
  resolveLLMProvider,
} from './forms/llmCatalog';

const configs = ref([]);
const testingId = ref(null);
const testingAll = ref(false);
const testingCurrent = ref(false);
const testResults = ref({}); // config_id -> { ok, message }
const loading = ref(false);
const saving = ref(false);
const showDialog = ref(false);
const editingConfig = ref(null);
const formRef = ref();

const form = reactive({
  name: '',
  config_id: '',
  provider: '',
  is_default: false,
  enabled: true,
  type: 'openai',
  model_name: 'gpt-3.5-turbo',
  api_key: '',
  base_url: 'https://api.openai.com/v1',
  max_tokens: 4000,
  temperature: 0.7,
  top_p: 0.9,
  thinking_mode: 'default',
  thinking_budget_tokens: null,
  thinking_effort: 'medium',
  thinking_clear_thinking: 'default',
  bot_id: '',
  user_prefix: '',
  connector_id: '1024',
});

const rules = {
  name: [{ required: true, message: 'Vui lòng nhập tên cấu hình', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Vui lòng nhập ID cấu hình', trigger: 'blur' }],
  provider: [{ required: true, message: 'Vui lòng chọn nhà cung cấp', trigger: 'change' }],
  model_name: [
    {
      required: true,
      message: 'Vui lòng nhập tên mô hình',
      trigger: 'change',
    },
    {
      validator: (_, value, callback) => {
        const provider = resolveLLMProvider(form.provider, form.type);
        const providerType = getProviderFixedType(provider);
        if ((providerType === 'openai' || providerType === 'ollama') && !value) {
          callback(new Error('Vui lòng nhập tên mô hình'));
          return;
        }
        callback();
      },
      trigger: 'change',
    },
  ],
  api_key: [
    {
      validator: (_, value, callback) => {
        const provider = resolveLLMProvider(form.provider, form.type);
        if (getProviderFixedType(provider) !== 'ollama' && !value) {
          callback(new Error('Vui lòng nhập khóa API'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
  base_url: [
    {
      validator: (_, value, callback) => {
        const provider = resolveLLMProvider(form.provider, form.type);
        if (isProviderBaseURLEditable(provider) && !value) {
          callback(new Error('Vui lòng nhập URL cơ sở'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
  max_tokens: [
    {
      validator: (_, value, callback) => {
        const provider = resolveLLMProvider(form.provider, form.type);
        const providerType = getProviderFixedType(provider);
        if (
          (providerType === 'openai' || providerType === 'ollama') &&
          (!value || Number(value) < 1 || Number(value) > 100000)
        ) {
          callback(new Error('max_tokens phải trong khoảng 1-100000'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
  bot_id: [
    {
      validator: (_, value, callback) => {
        const provider = resolveLLMProvider(form.provider, form.type);
        if (getProviderFixedType(provider) === 'coze' && !value) {
          callback(new Error('Vui lòng nhập Coze Bot ID'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
  temperature: [{ type: 'number', min: 0, max: 2, message: 'Nhiệt độ phải trong khoảng 0-2', trigger: 'blur' }],
  top_p: [{ type: 'number', min: 0, max: 1, message: 'Top P phải trong khoảng 0-1', trigger: 'blur' }],
};

const loadConfigs = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/llm-configs');
    configs.value = (response.data.data || []).map((row) => {
      const configObj = parseJsonData(row?.json_data);
      return {
        ...row,
        provider: resolveLLMProvider(row?.provider, configObj?.type),
      };
    });
  } catch (error) {
    ElMessage.error('Tải cấu hình thất bại');
  } finally {
    loading.value = false;
  }
};

const editConfig = (config) => {
  editingConfig.value = config;
  form.name = config.name;
  form.config_id = config.config_id;
  form.provider = config.provider;
  form.is_default = config.is_default;
  form.enabled = config.enabled;

  // Phân tích JSON cấu hình và điền vào các trường tương ứng
  try {
    const configObj = JSON.parse(config.json_data || '{}');
    const detectedProvider = resolveLLMProvider(config.provider, configObj.type);
    const detectedType = getProviderFixedType(detectedProvider);
    form.provider = detectedProvider;
    form.type = detectedType;
    form.model_name =
      configObj.model_name || (detectedType === 'coze' ? 'coze' : detectedType === 'dify' ? 'dify' : '');
    form.api_key = configObj.api_key || '';
    form.base_url =
      configObj.base_url ||
      (detectedType === 'coze' ? 'https://api.coze.com' : detectedType === 'dify' ? 'https://api.dify.ai/v1' : '');
    form.max_tokens = configObj.max_tokens || 4000;
    form.temperature = configObj.temperature || 0.7;
    form.top_p = configObj.top_p || 0.9;
    form.thinking_mode = configObj.thinking?.mode || 'default';
    form.thinking_budget_tokens =
      configObj.thinking?.budget_tokens !== undefined ? Number(configObj.thinking.budget_tokens) || null : null;
    form.thinking_effort = configObj.thinking?.effort || 'medium';
    form.thinking_clear_thinking =
      configObj.thinking?.clear_thinking !== undefined ? configObj.thinking.clear_thinking : 'default';
    form.bot_id = configObj.bot_id || '';
    form.user_prefix = configObj.user_prefix || '';
    form.connector_id = configObj.connector_id || '1024';
  } catch (error) {
    console.error('Phân tích JSON cấu hình thất bại:', error);
  }

  showDialog.value = true;
  nextTick(() => {
    formRef.value?.clearValidate?.();
  });
};

const openCreateDialog = () => {
  resetForm();
  showDialog.value = true;
  nextTick(() => {
    formRef.value?.clearValidate?.();
  });
};

const handleSave = async () => {
  if (!formRef.value) return;

  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true;
      try {
        // Kiểm tra có phải lần đầu thêm cấu hình không
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
          await api.put(`/admin/llm-configs/${editingConfig.value.id}`, configData);
          ElMessage.success('Cập nhật cấu hình thành công');
        } else {
          await api.post('/admin/llm-configs', configData);
          ElMessage.success('Tạo cấu hình thành công');
        }

        showDialog.value = false;
        loadConfigs();
      } catch (error) {
        ElMessage.error('Lưu thất bại: ' + (error.response?.data?.message || error.message));
      } finally {
        saving.value = false;
      }
    }
  });
};

const toggleEnable = async (config) => {
  try {
    await api.post(`/admin/configs/${config.id}/toggle`);
    ElMessage.success(`${config.enabled ? 'Kích hoạt' : 'Vô hiệu hoá'} thành công`);
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

    await api.put(`/admin/llm-configs/${config.id}`, configData);
    ElMessage.success(config.is_default ? 'Đặt làm mặc định thành công' : 'Hủy mặc định thành công');

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
  return r.first_packet_ms != null ? `${r.first_packet_ms}ms` : 'Đạt';
}
function formatTestResultTip(r) {
  if (!r?.ok) return '';
  const parts = [];
  if (r.first_packet_ms != null) parts.push(`Gói đầu ${r.first_packet_ms}ms`);
  if (r.reasoning_content_returned) parts.push('Phát hiện nội dung suy nghĩ từ hệ thống');
  return parts.length ? parts.join(', ') : 'Đạt';
}
function formatTestMessage(result) {
  const base = result.message || '';
  const suffix = [];
  if (result.first_packet_ms != null) suffix.push(`${result.first_packet_ms}ms`);
  if (result.reasoning_content_returned) suffix.push('Phát hiện nội dung suy nghĩ từ hệ thống');
  return suffix.length ? `${base} ${suffix.join(' · ')}` : base;
}

function formatDraftTestLabel(name, configId) {
  return name?.trim() || configId?.trim() || 'Cấu hình hiện tại';
}

function getThinkingLabel(row) {
  const config = parseJsonData(row?.json_data);
  const provider = resolveLLMProvider(row?.provider, config?.type);
  const mode = String(config?.thinking?.mode || '').trim();
  if (!mode || mode === 'default') {
    return '';
  }

  const thinkingConfig = getProviderThinkingConfig(provider, config?.model_name);
  const option = (thinkingConfig?.options || []).find((item) => item.value === mode);
  return option?.label || mode;
}

function getProviderLabel(provider) {
  const labels = {
    azure: 'Azure OpenAI',
    anthropic: 'Anthropic',
    zhipu: 'Zhipu AI',
    aliyun: 'Alibaba Cloud',
    doubao: 'Doubao',
    siliconflow: 'SiliconFlow',
    deepseek: 'DeepSeek',
    openai: 'OpenAI',
    ollama: 'Ollama',
    dify: 'Dify',
    coze: 'Coze',
  };
  return labels[provider] || provider;
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
    ElMessage.warning('Không có cấu hình nào đang được kích hoạt');
    return;
  }
  testingAll.value = true;
  testResults.value = {};
  let okCount = 0;
  try {
    for (const row of list) {
      try {
        const result = await testSingleConfig('llm', row.config_id);
        testResults.value = { ...testResults.value, [row.config_id]: result };
        if (result.ok) okCount++;
      } catch (_) {
        testResults.value = { ...testResults.value, [row.config_id]: { ok: false, message: 'Yêu cầu thất bại' } };
      }
    }
    ElMessage.success(`Kiểm tra hoàn tất: ${okCount}/${list.length} đạt`);
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
    const result = await testWithData('llm', { provider: configId, [configId]: payload });
    const label = formatDraftTestLabel(form.name, configId);
    if (result.ok) {
      ElMessage.success(`${label}: ${formatTestMessage(result) || 'Kiểm tra thành công'}`);
    } else {
      ElMessage.warning(`${label}: ${result.message || 'Kiểm tra không đạt'}`);
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Yêu cầu kiểm tra thất bại');
  } finally {
    testingCurrent.value = false;
  }
};

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Bạn có chắc muốn xóa cấu hình này không?', 'Thông báo', {
      confirmButtonText: 'Xác nhận',
      cancelButtonText: 'Hủy',
      type: 'warning',
    });

    await api.delete(`/admin/llm-configs/${id}`);
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
  form.type = 'openai';
  form.model_name = '';
  form.api_key = '';
  form.base_url = '';
  form.max_tokens = 4000;
  form.temperature = 0.7;
  form.top_p = 0.9;
  form.thinking_mode = 'default';
  form.thinking_budget_tokens = null;
  form.thinking_effort = 'medium';
  form.thinking_clear_thinking = 'default';
  form.bot_id = '';
  form.user_prefix = '';
  form.connector_id = '1024';
};

const handleDialogClose = () => {
  showDialog.value = false;
  resetForm();
  nextTick(() => {
    formRef.value?.clearValidate?.();
  });
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

.thinking-tag {
  max-width: 84px;
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
