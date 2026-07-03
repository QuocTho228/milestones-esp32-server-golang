<template>
  <div class="dashboard-page">
    <section class="stats-grid">
      <article class="metric-card">
        <div class="metric-top">
          <span class="metric-icon users">
            <el-icon><User /></el-icon>
          </span>
          <span class="metric-trend">{{ authStore.isAdmin ? 'Người dùng toàn cục' : 'Tài khoản được liên kết' }}</span>
        </div>
        <strong>{{ authStore.isAdmin ? stats.totalUsers : 1 }}</strong>
        <p>{{ authStore.isAdmin ? 'Tổng số người dùng' : 'Tài khoản đăng nhập hiện tại' }}</p>
      </article>

      <article class="metric-card">
        <div class="metric-top">
          <span class="metric-icon devices">
            <el-icon><Monitor /></el-icon>
          </span>
          <span class="metric-trend">Trực tuyến {{ stats.onlineDevices }}</span>
        </div>
        <strong>{{ stats.totalDevices }}</strong>
        <p>{{ authStore.isAdmin ? 'Tổng số thiết bị' : 'Thiết bị của tôi' }}</p>
      </article>

      <article class="metric-card">
        <div class="metric-top">
          <span class="metric-icon agents">
            <el-icon><Cpu /></el-icon>
          </span>
          <span class="metric-trend">Đang hoạt động</span>
        </div>
        <strong>{{ stats.totalAgents }}</strong>
        <p>{{ authStore.isAdmin ? 'Số lượng tác nhân' : 'Trợ lý AI của tôi' }}</p>
      </article>

      <article class="metric-card">
        <div class="metric-top">
          <span class="metric-icon status">
            <el-icon><Connection /></el-icon>
          </span>
          <span class="metric-trend">Giám sát thời gian thực</span>
        </div>
        <strong>{{ stats.onlineDevices }}</strong>
        <p>Thiết bị online</p>
      </article>
    </section>

    <section class="dashboard-grid" :class="{ compact: !authStore.isAdmin }">
      <div class="dashboard-main">
        <el-card v-if="authStore.isAdmin" class="dashboard-card service-card">
          <template #header>
            <div class="card-header">
              <div>
                <p class="card-eyebrow">SERVICE ADDRESS</p>
                <h3>Địa chỉ dịch vụ</h3>
              </div>
              <el-button type="warning" size="small" :loading="otaTestLoading" @click="runOtaTest">
                OTA Kiểm tra
              </el-button>
            </div>
          </template>

          <div v-loading="addressLoading" class="address-card-content">
            <template v-if="!addressLoading && (serviceAddress.otaUrl || serviceAddress.wsUrl)">
              <div class="address-list">
                <div class="address-row">
                  <span class="address-label">OTA</span>
                  <span class="address-value" :title="serviceAddress.otaUrl">{{ serviceAddress.otaUrl || '—' }}</span>
                  <el-button
                    v-if="serviceAddress.otaUrl"
                    link
                    type="primary"
                    :icon="CopyDocument"
                    @click="copyAddress(serviceAddress.otaUrl)"
                  />
                </div>
                <div class="address-row">
                  <span class="address-label">WS</span>
                  <span class="address-value" :title="serviceAddress.wsUrl">{{ serviceAddress.wsUrl || '—' }}</span>
                  <el-button
                    v-if="serviceAddress.wsUrl"
                    link
                    type="primary"
                    :icon="CopyDocument"
                    @click="copyAddress(serviceAddress.wsUrl)"
                  />
                </div>
                <div v-if="serviceAddress.mqttEndpoint" class="address-row">
                  <span class="address-label">MQTT</span>
                  <span class="address-value" :title="serviceAddress.mqttEndpoint">{{
                    serviceAddress.mqttEndpoint
                  }}</span>
                  <el-button
                    link
                    type="primary"
                    :icon="CopyDocument"
                    @click="copyAddress(serviceAddress.mqttEndpoint)"
                  />
                </div>
                <div v-if="serviceAddress.udpAddress" class="address-row">
                  <span class="address-label">UDP</span>
                  <span class="address-value" :title="serviceAddress.udpAddress">{{ serviceAddress.udpAddress }}</span>
                  <el-button link type="primary" :icon="CopyDocument" @click="copyAddress(serviceAddress.udpAddress)" />
                </div>
              </div>

              <div v-if="otaTestResult !== null" class="ota-test-block">
                <span class="apple-chip is-primary">Phản hồi OTA</span>
                <pre class="ota-test-pre">{{ otaTestResult }}</pre>
              </div>
            </template>

            <div v-else-if="!addressLoading" class="empty-inline">Không có cấu hình OTA nào khả dụng</div>
          </div>
        </el-card>

        <el-card v-if="authStore.isAdmin" class="dashboard-card">
          <template #header>
            <div class="card-header">
              <div>
                <p class="card-eyebrow">CONFIGURATION</p>
                <h3>Quản lý cấu hình</h3>
              </div>
            </div>
          </template>

          <div class="config-actions">
            <button class="action-card action-primary" type="button" @click="$router.push('/admin/config-wizard')">
              <span class="action-icon"
                ><el-icon><Guide /></el-icon
              ></span>
              <span class="action-copy">
                <strong>Trình hướng dẫn cấu hình</strong>
                <small>Thực hiện cấu hình lần đầu hoặc cập nhật cấu hình theo quy trình thống nhất</small>
              </span>
            </button>

            <button class="action-card" type="button" @click="exportConfig">
              <span class="action-icon"
                ><el-icon><Download /></el-icon
              ></span>
              <span class="action-copy">
                <strong>Xuất cấu hình</strong>
                <small>Tải xuống cấu hình hợp lệ hiện tại làm bản sao lưu</small>
              </span>
            </button>

            <button class="action-card" type="button" @click="importConfig">
              <span class="action-icon"
                ><el-icon><Upload /></el-icon
              ></span>
              <span class="action-copy">
                <strong>Nhập cấu hình</strong>
                <small>Hỗ trợ nhập nhanh YAML/JSON</small>
              </span>
            </button>
          </div>

          <input
            ref="fileInput"
            type="file"
            accept=".yaml,.yml,.json"
            style="display: none"
            @change="handleFileChange"
          />
        </el-card>
      </div>

      <div class="dashboard-side">
        <el-card class="dashboard-card info-card">
          <template #header>
            <div class="card-header">
              <div>
                <p class="card-eyebrow">SYSTEM</p>
                <h3>Thông tin hệ thống</h3>
              </div>
            </div>
          </template>

          <div class="info-list">
            <div class="info-row">
              <span>Phiên bản hệ thống</span>
              <strong>v1.0.0</strong>
            </div>
            <div class="info-row">
              <span>Thời gian bắt đầu chương trình</span>
              <strong>{{ programStartedAt }}</strong>
            </div>
            <div class="info-row">
              <span>Người dùng hiện tại</span>
              <strong>{{ authStore.user?.username || '—' }}</strong>
            </div>
            <div class="info-row">
              <span>Vai trò người dùng</span>
              <el-tag :type="authStore.isAdmin ? 'danger' : 'primary'" effect="light">
                {{ authStore.isAdmin ? 'Quản trị viên' : 'Người dùng' }}
              </el-tag>
            </div>
          </div>
        </el-card>

        <el-card class="dashboard-card quick-card">
          <template #header>
            <div class="card-header">
              <div>
                <p class="card-eyebrow">SHORTCUTS</p>
                <h3>Thao tác nhanh</h3>
              </div>
            </div>
          </template>

          <div class="quick-actions">
            <template v-if="authStore.isAdmin">
              <button class="quick-action" type="button" @click="$router.push('/admin/users')">
                <span class="quick-action-icon"
                  ><el-icon><User /></el-icon
                ></span>
                <span>
                  <strong>Quản lý người dùng</strong>
                  <small>Xem tài khoản, quyền và trạng thái</small>
                </span>
              </button>
              <button class="quick-action" type="button" @click="$router.push('/admin/llm-config')">
                <span class="quick-action-icon"
                  ><el-icon><Setting /></el-icon
                ></span>
                <span>
                  <strong>Cấu hình LLM</strong>
                  <small>Điều chỉnh kết nối mô hình, siêu tham số và chiến lược</small>
                </span>
              </button>
              <button class="quick-action" type="button" @click="$router.push('/admin/vad-config')">
                <span class="quick-action-icon"
                  ><el-icon><Cpu /></el-icon
                ></span>
                <span>
                  <strong>Cấu hình VAD</strong>
                  <small>Điều chỉnh phát hiện hoạt động âm thanh và thời gian thực</small>
                </span>
              </button>
            </template>

            <template v-else>
              <button class="quick-action" type="button" @click="$router.push('/agents')">
                <span class="quick-action-icon"
                  ><el-icon><Monitor /></el-icon
                ></span>
                <span>
                  <strong>Quản lý tác nhân AI</strong>
                  <small>Duy trì cài đặt vai trò, thiết bị ràng buộc và khả năng kiểm tra</small>
                </span>
              </button>
              <div class="empty-inline">
                Các thao tác thường dùng của người dùng chủ yếu tập trung ở mục Tác nhân AI và Bảng điều khiển thiết bị
              </div>
            </template>
          </div>
        </el-card>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { useAuthStore } from '@/stores/auth';
import api from '@/utils/api';
import { ElMessage } from 'element-plus';
import {
  User,
  Monitor,
  Connection,
  Setting,
  Download,
  Upload,
  Cpu,
  Guide,
  CopyDocument,
} from '@element-plus/icons-vue';

const authStore = useAuthStore();

const addressLoading = ref(false);
const serviceAddress = ref({
  otaUrl: '',
  wsUrl: '',
  mqttEndpoint: '',
  udpAddress: '',
});

async function loadServiceAddress() {
  addressLoading.value = true;
  serviceAddress.value = {
    otaUrl: '',
    wsUrl: '',
    mqttEndpoint: '',
    udpAddress: '',
  };
  try {
    const [otaRes, udpRes] = await Promise.all([api.get('/admin/ota-configs'), api.get('/admin/udp-configs')]);
    const otaList = otaRes.data?.data || [];
    const config = otaList.find((c) => c.is_default) || otaList[0];
    if (config?.json_data) {
      const data = JSON.parse(config.json_data || '{}');

      let envData = data.external || {};
      const hasExternalWs = envData.websocket?.url;
      const hasExternalOta = envData.ota_url;
      if (!hasExternalWs && !hasExternalOta) {
        envData = data.test || {};
      }

      let otaUrl = envData.ota_url || '';
      if (!otaUrl) {
        const wsUrl = envData.websocket?.url || '';
        if (wsUrl) {
          const matched = wsUrl.match(/^(wss?):\/\/([^:/]+)(?::(\d+))?/);
          if (matched) {
            const protocol = matched[1] === 'wss' ? 'https' : 'http';
            const port = matched[3] || (matched[1] === 'wss' ? '443' : '80');
            otaUrl = `${protocol}://${matched[2]}:${port}/milestones/ota/`;
          }
        }
      }
      serviceAddress.value.otaUrl = otaUrl;
      serviceAddress.value.wsUrl = envData.websocket?.url || '';

      const mqttEnabled = envData.mqtt?.enable;
      const endpoint = envData.mqtt?.endpoint || '';
      if (mqttEnabled && endpoint) {
        serviceAddress.value.mqttEndpoint = endpoint;
      }
    }

    const udpList = udpRes.data?.data || [];
    const udpConfig = udpList.find((c) => c.is_default) || udpList[0];
    if (udpConfig?.json_data) {
      const udpData = JSON.parse(udpConfig.json_data || '{}');
      const host = udpData.external_host || '';
      const port = udpData.external_port;
      if (host && port != null) {
        serviceAddress.value.udpAddress = `${host}:${port}`;
      }
    }
  } catch (err) {
    console.error('Không tải được địa chỉ dịch vụ:', err);
  } finally {
    addressLoading.value = false;
  }
}

function copyAddress(text) {
  if (!text) return;
  navigator.clipboard
    .writeText(text)
    .then(() => {
      ElMessage.success('Đã sao chép vào bảng nhớ tạm');
    })
    .catch(() => {
      ElMessage.error('Sao chép không thành công');
    });
}

const otaTestLoading = ref(false);
const otaTestResult = ref(null);

function formatOtaResponseDisplay(str) {
  if (str == null || str === '') return '';
  const content = String(str).trim();
  if (!content) return '';
  try {
    return JSON.stringify(JSON.parse(content), null, 2);
  } catch {
    return content;
  }
}

async function runOtaTest() {
  otaTestLoading.value = true;
  otaTestResult.value = null;
  try {
    const res = await api.post('/admin/configs/test', { types: ['ota'] }, { timeout: 30000 });
    const data = res.data?.data ?? res.data;
    const ota = data?.ota;
    if (ota && typeof ota === 'object') {
      const entry = Object.entries(ota).find(([key]) => !key.startsWith('_'));
      if (entry) {
        const [, value] = entry;
        let displayText = '';

        if (value.websocket) {
          const ws = value.websocket;
          displayText += `WebSocket: ${ws.ok ? '✓' : '✗'} ${ws.message}`;
          displayText += ws.first_packet_ms != null ? ` (${ws.first_packet_ms}ms)\n` : '\n';
        }

        if (value.mqtt_udp) {
          const mqtt = value.mqtt_udp;
          displayText += `MQTT UDP: ${mqtt.ok ? '✓' : '✗'} ${mqtt.message}`;
          displayText += mqtt.first_packet_ms != null ? ` (${mqtt.first_packet_ms}ms)\n` : '\n';
        }

        if (value.ota_response !== undefined && value.ota_response !== '') {
          displayText += `\n--- Phản hồi OTA ---\n${formatOtaResponseDisplay(value.ota_response)}`;
        }

        otaTestResult.value = displayText.trim() || 'Không có thông tin chi tiết được lấy';
        ElMessage[value.ok ? 'success' : 'warning'](
          value.message || (value.ok ? 'Kiểm tra OTA thành công' : 'Kiểm tra OTA không thành công'),
        );
      } else {
        otaTestResult.value = 'Không có kết quả kiểm tra OTA';
      }
    } else {
      otaTestResult.value = typeof data === 'string' ? data : JSON.stringify(data || {}, null, 2);
    }
  } catch (error) {
    const errorMsg =
      error.response?.data && typeof error.response.data === 'object'
        ? JSON.stringify(error.response.data, null, 2)
        : error.response?.data?.message || error.message || 'Yêu cầu không thành công';
    otaTestResult.value = errorMsg;
    ElMessage.error('Yêu cầu kiểm tra OTA không thành công');
  } finally {
    otaTestLoading.value = false;
  }
}

const stats = ref({
  totalUsers: 0,
  totalDevices: 0,
  totalAgents: 0,
  onlineDevices: 0,
});

const programStartedAt = ref('—');
const fileInput = ref(null);

onMounted(async () => {
  await loadStats();
  if (authStore.isAdmin) {
    loadServiceAddress();
  }
});

const loadStats = async () => {
  try {
    const response = await api.get('/dashboard/stats');
    stats.value = {
      totalUsers: response.data.totalUsers || 0,
      totalDevices: response.data.totalDevices || 0,
      totalAgents: response.data.totalAgents || 0,
      onlineDevices: response.data.onlineDevices || 0,
    };
    programStartedAt.value = response.data?.programStartedAt
      ? new Date(response.data.programStartedAt).toLocaleString('zh-CN')
      : '—';
  } catch (error) {
    console.error('Không thể tải số liệu thống kê:', error);
    stats.value = {
      totalUsers: 0,
      totalDevices: 0,
      totalAgents: 0,
      onlineDevices: 0,
    };
    programStartedAt.value = '—';
  }
};

const exportConfig = async () => {
  try {
    const response = await fetch('/api/admin/configs/export', {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
    });

    if (response.ok) {
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'config.yaml';
      document.body.appendChild(link);
      link.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(link);
      ElMessage.success('Xuất cấu hình thành công');
    } else {
      ElMessage.error('Xuất cấu hình thất bại');
    }
  } catch (error) {
    console.error('Không thể xuất cấu hình:', error);
    ElMessage.error('Xuất cấu hình thất bại');
  }
};

const importConfig = () => {
  fileInput.value.click();
};

const handleFileChange = async (event) => {
  const file = event.target.files[0];
  if (!file) return;

  const validExtensions = ['.yaml', '.yml', '.json'];
  const fileExtension = file.name.toLowerCase().substring(file.name.lastIndexOf('.'));

  if (!validExtensions.includes(fileExtension)) {
    ElMessage.error('Vui lòng chọn tệp ở định dạng YAML hoặc JSON');
    return;
  }

  const formData = new FormData();
  formData.append('file', file);

  try {
    const response = await fetch('/api/admin/configs/import', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${authStore.token}`,
      },
      body: formData,
    });

    if (response.ok) {
      ElMessage.success('Nhập cấu hình thành công');
    } else {
      const error = await response.json();
      ElMessage.error(error.error || 'Nhập cấu hình không thành công');
    }
  } catch (error) {
    console.error('Quá trình nhập cấu hình thất bại:', error);
    ElMessage.error('Nhập cấu hình thất bại');
  }

  event.target.value = '';
};
</script>

<style scoped>
.dashboard-page {
  display: grid;
  gap: 20px;
}
.card-eyebrow {
  margin: 0;
  color: var(--apple-text-tertiary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.metric-card {
  padding: 22px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(255, 255, 255, 0.88);
  box-shadow: var(--apple-shadow-md);
}

.metric-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.metric-icon {
  width: 42px;
  height: 42px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.metric-icon.users {
  color: var(--apple-primary);
  background: var(--apple-primary-soft);
}

.metric-icon.devices {
  color: #176a31;
  background: var(--apple-success-soft);
}

.metric-icon.agents {
  color: #875f00;
  background: var(--apple-warning-soft);
}

.metric-icon.status {
  color: #8a1f19;
  background: var(--apple-danger-soft);
}

.metric-trend {
  color: var(--apple-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.metric-card strong {
  display: block;
  font-size: 34px;
  line-height: 1;
  letter-spacing: -0.05em;
}

.metric-card p {
  margin: 10px 0 0;
  color: var(--apple-text-secondary);
  font-size: 14px;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(320px, 0.9fr);
  gap: 18px;
}

.dashboard-grid.compact {
  grid-template-columns: 1fr 360px;
}

.dashboard-main,
.dashboard-side {
  display: grid;
  gap: 18px;
}

.dashboard-card :deep(.el-card__header) {
  padding-bottom: 18px;
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.card-header h3 {
  margin: 4px 0 0;
  font-size: 18px;
}

.address-card-content {
  display: grid;
  gap: 16px;
}

.address-list {
  display: grid;
  gap: 12px;
}

.address-row {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(248, 250, 252, 0.86);
  border: 1px solid rgba(229, 229, 234, 0.76);
}

.address-label {
  color: var(--apple-text-tertiary);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.address-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--apple-text);
  font-weight: 500;
}

.ota-test-block {
  display: grid;
  gap: 10px;
}

.ota-test-pre {
  margin: 0;
  padding: 16px;
  border-radius: 18px;
  background: #f7f9fc;
  border: 1px solid rgba(229, 229, 234, 0.72);
  color: #445064;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 180px;
  overflow: auto;
}

.config-actions,
.quick-actions {
  display: grid;
  gap: 12px;
}

.action-card,
.quick-action {
  width: 100%;
  padding: 16px;
  border: 1px solid rgba(229, 229, 234, 0.76);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.9);
  display: flex;
  align-items: center;
  gap: 14px;
  text-align: left;
  cursor: pointer;
  color: inherit;
}

.action-card:hover,
.quick-action:hover {
  transform: translateY(-1px);
  box-shadow: var(--apple-shadow-sm);
  border-color: rgba(0, 122, 255, 0.18);
}

.action-primary {
  background: linear-gradient(180deg, rgba(0, 122, 255, 0.12) 0%, rgba(0, 122, 255, 0.06) 100%);
}

.action-icon,
.quick-action-icon {
  width: 42px;
  height: 42px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 122, 255, 0.1);
  color: var(--apple-primary);
  font-size: 18px;
  flex: none;
}

.action-copy,
.quick-action span:last-child {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.action-copy strong,
.quick-action strong {
  font-size: 15px;
}

.action-copy small,
.quick-action small {
  color: var(--apple-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.info-list {
  display: grid;
  gap: 12px;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid rgba(229, 229, 234, 0.72);
}

.info-row:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}

.info-row span {
  color: var(--apple-text-secondary);
  font-size: 14px;
}

.info-row strong {
  color: var(--apple-text);
  font-size: 14px;
}

.empty-inline {
  color: var(--apple-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

@media (max-width: 1280px) {
  .dashboard-grid,
  .dashboard-grid.compact {
    grid-template-columns: 1fr;
  }

  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .address-row {
    grid-template-columns: 1fr;
    align-items: flex-start;
  }
}
</style>
