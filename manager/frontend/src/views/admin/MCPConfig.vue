<template>
  <div class="mcp-config">
    <el-form ref="formRef" :model="form" :rules="rules" class="config-form" v-loading="loading">
      <div class="config-layout">
        <el-card class="config-card config-card-main" shadow="never">
          <template #header>
            <div class="card-head">
              <div>
                <p class="card-kicker">Global MCP</p>
                <h3>Dịch vụ MCP toàn cục</h3>
                <p class="card-description">
                  Quản lý các máy chủ MCP dùng chung phía server, chiến lược kết nối lại và phạm vi công cụ được phép.
                </p>
              </div>
              <el-tag :type="form.mcp.global.enabled ? 'success' : 'info'" effect="plain" round>
                {{ form.mcp.global.enabled ? `${enabledServerCount} dịch vụ đang bật` : 'MCP toàn cục đã tắt' }}
              </el-tag>
            </div>
          </template>

          <div class="field-grid field-grid-main">
            <el-form-item label="Bật MCP toàn cục" prop="mcp.global.enabled">
              <div class="switch-field">
                <div>
                  <div class="switch-title">Cho phép server kết nối MCP tập trung</div>
                  <div class="field-help">
                    Khi tắt sẽ không chủ động thiết lập kết nối MCP toàn cục, nhưng MCP cục bộ vẫn có thể kiểm soát
                    riêng lẻ.
                  </div>
                </div>
                <el-switch v-model="form.mcp.global.enabled" />
              </div>
            </el-form-item>

            <el-form-item label="Thời gian kết nối lại (giây)" prop="mcp.global.reconnect_interval">
              <el-input-number
                v-model="form.mcp.global.reconnect_interval"
                :min="1"
                :max="3600"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>

            <el-form-item label="Số lần kết nối lại tối đa" prop="mcp.global.max_reconnect_attempts">
              <el-input-number
                v-model="form.mcp.global.max_reconnect_attempts"
                :min="1"
                :max="100"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </div>

          <div class="server-list">
            <div class="server-list-header">
              <div>
                <h4>Danh sách máy chủ</h4>
                <p>
                  Mỗi máy chủ có thể bật/tắt riêng lẻ, dò tìm công cụ và giới hạn tập công cụ hiển thị cho chương trình
                  chính.
                </p>
              </div>
              <el-button type="primary" @click="addGlobalServer">
                <el-icon><Plus /></el-icon>
                Thêm máy chủ
              </el-button>
            </div>

            <div v-if="form.mcp.global.servers.length === 0" class="empty-state">
              <strong>Chưa có máy chủ MCP nào</strong>
              <p>
                Hãy thêm một máy chủ, điền tên, loại và URL; để trống danh sách công cụ cho phép có nghĩa là toàn bộ
                công cụ của máy chủ đó đều khả dụng.
              </p>
            </div>

            <div v-for="(server, index) in form.mcp.global.servers" :key="index" class="server-item">
              <div class="server-item-header">
                <div class="server-title-row">
                  <strong>Máy chủ {{ index + 1 }}</strong>
                  <el-tag size="small" :type="server.enabled ? 'success' : 'info'" effect="plain" round>
                    {{ server.enabled ? 'Đang bật' : 'Đã tắt' }}
                  </el-tag>
                  <el-tag size="small" :type="server.allowed_tools?.length ? 'warning' : 'info'" effect="plain" round>
                    {{ server.allowed_tools?.length ? `${server.allowed_tools.length} công cụ` : 'Tất cả công cụ' }}
                  </el-tag>
                </div>

                <div class="server-actions">
                  <el-button size="small" :loading="server._tools_loading" @click="discoverGlobalServerTools(server)">
                    Dò tìm công cụ
                  </el-button>
                  <el-button size="small" type="danger" @click="removeGlobalServer(index)">
                    <el-icon><Delete /></el-icon>
                    Xóa
                  </el-button>
                </div>
              </div>

              <div class="field-grid server-grid">
                <el-form-item :label="'Tên máy chủ'" :prop="`mcp.global.servers.${index}.name`">
                  <el-input v-model="server.name" placeholder="Ví dụ: Amap MCP" />
                </el-form-item>

                <el-form-item :label="'Loại máy chủ'" :prop="`mcp.global.servers.${index}.type`">
                  <el-select v-model="server.type" placeholder="Chọn loại máy chủ" style="width: 100%">
                    <el-option label="SSE" value="sse" />
                    <el-option label="StreamableHTTP" value="streamablehttp" />
                  </el-select>
                </el-form-item>

                <el-form-item :label="'URL máy chủ'" :prop="`mcp.global.servers.${index}.url`" class="field-span-full">
                  <el-input v-model="server.url" placeholder="Ví dụ: https://example.com/mcp" />
                </el-form-item>

                <el-form-item :label="'Trạng thái kích hoạt'" :prop="`mcp.global.servers.${index}.enabled`">
                  <div class="switch-field">
                    <div>
                      <div class="switch-title">Cho phép chương trình kết nối dịch vụ này</div>
                      <div class="field-help">
                        Khi tắt, dịch vụ này sẽ không tham gia vào việc phát hiện và gọi công cụ toàn cục.
                      </div>
                    </div>
                    <el-switch v-model="server.enabled" />
                  </div>
                </el-form-item>
              </div>

              <el-form-item :label="'Công cụ được phép'" class="tool-form-item">
                <div class="tool-picker">
                  <div class="field-help">
                    Để trống cho phép tất cả công cụ của máy chủ đó. Khi dò tìm sẽ dùng loại và URL hiện tại.
                  </div>
                  <el-select
                    v-model="server.allowed_tools"
                    multiple
                    filterable
                    clearable
                    collapse-tags
                    collapse-tags-tooltip
                    style="width: 100%"
                    placeholder="Không chọn thì cho phép tất cả công cụ"
                    :loading="server._tools_loading"
                  >
                    <el-option
                      v-for="tool in server._tool_options"
                      :key="tool.name"
                      :label="tool.name"
                      :value="tool.name"
                    >
                      <div class="tool-option-row">
                        <span class="tool-option-name">{{ tool.name }}</span>
                        <span class="tool-option-desc">{{ tool.description || 'Không có mô tả' }}</span>
                      </div>
                    </el-option>
                  </el-select>
                </div>
              </el-form-item>
            </div>
          </div>
        </el-card>

        <el-card class="config-card config-card-side" shadow="never">
          <template #header>
            <div class="card-head">
              <div>
                <p class="card-kicker">Local MCP</p>
                <h3>Khả năng MCP cục bộ</h3>
                <p class="card-description">
                  Đây là các công tắc năng lực cơ bản mà chương trình chính cung cấp cho model, có thể kiểm soát từng
                  mục theo từng tình huống.
                </p>
              </div>
            </div>
          </template>

          <div class="field-stack">
            <el-form-item label="Thoát hội thoại" prop="local_mcp.exit_conversation">
              <div class="switch-field">
                <div>
                  <div class="switch-title">Cho phép model kết thúc phiên hội thoại hiện tại</div>
                  <div class="field-help">Phù hợp với các chuỗi công cụ cần chủ động đóng phiên hội thoại.</div>
                </div>
                <el-switch v-model="form.local_mcp.exit_conversation" />
              </div>
            </el-form-item>

            <el-form-item label="Xóa lịch sử hội thoại" prop="local_mcp.clear_conversation_history">
              <div class="switch-field">
                <div>
                  <div class="switch-title">Cho phép model xóa toàn bộ ngữ cảnh hiện tại</div>
                  <div class="field-help">Phù hợp khi chuyển đổi nhiệm vụ hoặc chủ động đặt lại ngữ cảnh.</div>
                </div>
                <el-switch v-model="form.local_mcp.clear_conversation_history" />
              </div>
            </el-form-item>

            <el-form-item label="Phát nhạc" prop="local_mcp.play_music">
              <div class="switch-field">
                <div>
                  <div class="switch-title">Cho phép model kích hoạt phát nhạc</div>
                  <div class="field-help">
                    Nếu sản phẩm của bạn không cần tính năng giải trí âm thanh, có thể tắt đi.
                  </div>
                </div>
                <el-switch v-model="form.local_mcp.play_music" />
              </div>
            </el-form-item>
          </div>
        </el-card>
      </div>

      <div class="footer-bar">
        <p class="footer-note">
          Sau khi lưu sẽ cập nhật cấu hình MCP toàn cục mặc định; nếu một máy chủ chỉ muốn hiển thị một số công cụ, hãy
          dò tìm công cụ trước rồi mới giới hạn danh sách cho phép.
        </p>
        <div class="footer-actions">
          <el-button plain :loading="loading" @click="loadConfig">Đặt lại về cấu hình hiện tại</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">Lưu cấu hình</el-button>
        </div>
      </div>
    </el-form>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Plus, Delete } from '@element-plus/icons-vue';
import api from '@/utils/api';

const loading = ref(false);
const saving = ref(false);
const configId = ref(null);
const formRef = ref();

const createDefaultState = () => ({
  mcp: {
    global: {
      enabled: true,
      servers: [],
      reconnect_interval: 300,
      max_reconnect_attempts: 10,
    },
  },
  local_mcp: {
    exit_conversation: true,
    clear_conversation_history: true,
    play_music: false,
  },
});

const form = reactive(createDefaultState());

const rules = {
  'mcp.global.reconnect_interval': [
    { required: true, message: 'Vui lòng nhập khoảng thời gian kết nối lại', trigger: 'blur' },
    {
      type: 'number',
      min: 1,
      max: 3600,
      message: 'Khoảng thời gian kết nối lại phải trong khoảng 1-3600',
      trigger: 'blur',
    },
  ],
  'mcp.global.max_reconnect_attempts': [
    { required: true, message: 'Vui lòng nhập số lần kết nối lại tối đa', trigger: 'blur' },
    { type: 'number', min: 1, max: 100, message: 'Số lần kết nối lại tối đa phải trong khoảng 1-100', trigger: 'blur' },
  ],
};

const createGlobalServer = () => ({
  name: '',
  type: 'streamablehttp',
  url: '',
  enabled: true,
  allowed_tools: [],
  _tool_options: [],
  _tools_loading: false,
});

const mergeServerToolOptions = (server, tools = []) => {
  const merged = new Map();

  (tools || []).forEach((tool) => {
    if (!tool?.name) return;
    merged.set(tool.name, {
      name: tool.name,
      description: tool.description || '',
    });
  });
  (server.allowed_tools || []).forEach((name) => {
    if (!name || merged.has(name)) return;
    merged.set(name, {
      name,
      description: 'Đang được chọn',
    });
  });

  server._tool_options = Array.from(merged.values()).sort((a, b) => a.name.localeCompare(b.name));
};

const normalizeGlobalServer = (server = {}) => {
  const normalized = {
    ...server,
    name: server.name || '',
    type: server.type || 'streamablehttp',
    url: server.url || '',
    enabled: server.enabled !== false,
    allowed_tools: Array.isArray(server.allowed_tools) ? [...server.allowed_tools] : [],
    _tool_options: [],
    _tools_loading: false,
  };
  mergeServerToolOptions(normalized);
  return normalized;
};

const enabledServerCount = computed(() => form.mcp.global.servers.filter((server) => server.enabled).length);

const resetForm = () => {
  const defaults = createDefaultState();
  form.mcp.global.enabled = defaults.mcp.global.enabled;
  form.mcp.global.reconnect_interval = defaults.mcp.global.reconnect_interval;
  form.mcp.global.max_reconnect_attempts = defaults.mcp.global.max_reconnect_attempts;
  form.mcp.global.servers = defaults.mcp.global.servers;
  form.local_mcp.exit_conversation = defaults.local_mcp.exit_conversation;
  form.local_mcp.clear_conversation_history = defaults.local_mcp.clear_conversation_history;
  form.local_mcp.play_music = defaults.local_mcp.play_music;
};

const addGlobalServer = () => {
  form.mcp.global.servers.push(createGlobalServer());
};

const removeGlobalServer = (index) => {
  form.mcp.global.servers.splice(index, 1);
};

const sanitizeGlobalServers = () => {
  return form.mcp.global.servers.map((server) => {
    const sanitized = { ...server };
    delete sanitized._tool_options;
    delete sanitized._tools_loading;
    return sanitized;
  });
};

const generateConfig = () => {
  return JSON.stringify({
    mcp: {
      global: {
        ...form.mcp.global,
        servers: sanitizeGlobalServers(),
      },
    },
    local_mcp: { ...form.local_mcp },
  });
};

const discoverGlobalServerTools = async (server) => {
  if (!server?.url) {
    ElMessage.warning('Vui lòng điền URL máy chủ trước');
    return;
  }

  server._tools_loading = true;
  try {
    const response = await api.post('/admin/mcp-configs/discover-tools', {
      transport: server.type,
      url: server.url,
      headers: server.headers || null,
    });
    mergeServerToolOptions(server, response.data?.data?.tools || []);
    ElMessage.success(`Đã dò tìm được ${server._tool_options.length} công cụ`);
  } catch (error) {
    mergeServerToolOptions(server);
    ElMessage.error(error.response?.data?.error || 'Dò tìm công cụ thất bại');
  } finally {
    server._tools_loading = false;
  }
};

const loadConfig = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/mcp-configs');
    const configs = response.data?.data || [];

    resetForm();

    if (configs.length > 0) {
      const config = configs.find((item) => item.is_default) || configs[0];
      configId.value = config.id;

      try {
        const configData = JSON.parse(config.json_data || '{}');
        if (configData.global && !configData.mcp) {
          form.mcp.global = {
            ...form.mcp.global,
            ...configData.global,
            servers: Array.isArray(configData.global?.servers)
              ? configData.global.servers.map(normalizeGlobalServer)
              : [],
          };
        } else if (configData.mcp?.global) {
          form.mcp.global = {
            ...form.mcp.global,
            ...configData.mcp.global,
            servers: Array.isArray(configData.mcp.global?.servers)
              ? configData.mcp.global.servers.map(normalizeGlobalServer)
              : [],
          };
        }

        if (configData.local_mcp) {
          Object.assign(form.local_mcp, configData.local_mcp);
        }
      } catch (error) {
        ElMessage.warning('Định dạng cấu hình MCP bất thường, đã khôi phục về giá trị mặc định');
      }
    } else {
      configId.value = null;
    }
  } catch (error) {
    ElMessage.error('Tải cấu hình MCP thất bại');
  } finally {
    loading.value = false;
  }
};

const handleSave = async () => {
  if (!formRef.value) return;

  try {
    await formRef.value.validate();
  } catch {
    return;
  }

  saving.value = true;
  try {
    const payload = {
      name: 'Cấu hình MCP toàn cục',
      config_id: 'mcp_global_config',
      is_default: true,
      json_data: generateConfig(),
    };

    if (configId.value) {
      await api.put(`/admin/mcp-configs/${configId.value}`, payload);
      ElMessage.success('Đã cập nhật cấu hình MCP');
    } else {
      const response = await api.post('/admin/mcp-configs', payload);
      configId.value = response.data?.data?.id || configId.value;
      ElMessage.success('Đã lưu cấu hình MCP');
    }

    await loadConfig();
  } catch (error) {
    ElMessage.error(error.response?.data?.message || 'Lưu cấu hình MCP thất bại');
  } finally {
    saving.value = false;
  }
};

onMounted(() => {
  loadConfig();
});
</script>

<style scoped>
.mcp-config {
  padding: 0 24px 32px;
}

.config-form {
  display: grid;
  gap: 24px;
}

.config-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(340px, 0.9fr);
  gap: 24px;
}

.config-card {
  border: 1px solid rgba(255, 255, 255, 0.88);
  background: rgba(255, 255, 255, 0.88);
  box-shadow: var(--apple-shadow-md);
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.card-kicker {
  display: block;
  margin: 0;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--apple-text-tertiary);
}

.card-head h3 {
  margin: 8px 0 0;
  font-size: 22px;
  line-height: 1.15;
  letter-spacing: -0.03em;
  color: var(--apple-text);
}

.card-description,
.field-help,
.footer-note,
.server-list-header p,
.empty-state p {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--apple-text-secondary);
}

.field-grid {
  display: grid;
  gap: 20px 18px;
}

.field-grid-main {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.field-stack {
  display: grid;
  gap: 20px;
}

.field-span-full {
  grid-column: 1 / -1;
}

.switch-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px 18px;
  align-items: center;
}

.switch-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--apple-text);
}

.server-list {
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid rgba(229, 229, 234, 0.72);
}

.server-list-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 18px;
}

.server-list-header h4,
.empty-state strong {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--apple-text);
}

.empty-state {
  padding: 18px;
  border-radius: 18px;
  border: 1px dashed rgba(229, 229, 234, 0.9);
  background: rgba(248, 250, 252, 0.72);
}

.server-item {
  padding: 18px;
  border-radius: 18px;
  border: 1px solid rgba(229, 229, 234, 0.88);
  background: rgba(248, 250, 252, 0.82);
}

.server-item + .server-item {
  margin-top: 16px;
}

.server-item-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 18px;
}

.server-title-row,
.server-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.server-title-row strong {
  font-size: 15px;
  color: var(--apple-text);
}

.server-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.tool-form-item {
  margin-top: 16px;
}

.tool-picker {
  width: 100%;
}

.tool-option-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.35;
}

.tool-option-name {
  color: var(--apple-text);
}

.tool-option-desc {
  color: var(--apple-text-secondary);
  font-size: 12px;
}

.footer-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 0 4px;
}

.footer-note {
  max-width: 680px;
  margin: 0;
}

.footer-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 12px;
}

:deep(.el-card__header) {
  padding: 24px 24px 0;
  border-bottom: none;
  background: transparent;
}

:deep(.el-card__body) {
  padding: 24px;
}

:deep(.el-form-item) {
  margin-bottom: 0;
}

:deep(.el-form-item__label) {
  font-size: 14px;
  font-weight: 600;
  color: var(--apple-text);
}

@media (max-width: 1180px) {
  .config-layout,
  .field-grid-main {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .mcp-config {
    padding: 0 16px 24px;
  }

  :deep(.el-card__body) {
    padding: 20px;
  }

  :deep(.el-card__header) {
    padding: 20px 20px 0;
  }

  .server-list-header,
  .server-item-header,
  .footer-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .server-grid {
    grid-template-columns: 1fr;
  }

  .footer-actions {
    justify-content: stretch;
  }

  .footer-actions :deep(.el-button) {
    flex: 1;
  }
}
</style>
