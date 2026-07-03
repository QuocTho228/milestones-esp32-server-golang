<template>
  <div class="ota-config">
    <el-form ref="formRef" :model="form" :rules="rules" class="config-form" v-loading="loading">
      <el-card class="config-card" shadow="never">
        <template #header>
          <div class="card-head">
            <div>
              <p class="card-kicker">OTA Cơ bản</p>
              <h3>Chữ ký và ràng buộc cơ bản</h3>
              <p class="card-description">
                Mật khẩu người dùng MQTT được phân phối qua OTA sẽ dựa trên khóa ký, vui lòng đảm bảo giống với cấu hình
                MQTT Server.
              </p>
            </div>
          </div>
        </template>

        <div class="field-grid">
          <el-form-item label="Khóa chữ ký" prop="signature_key" class="field-span-full">
            <el-input
              v-model="form.signature_key"
              placeholder="Nhập khóa chữ ký dùng chung cho OTA và MQTT Server"
              show-password
            />
            <div class="field-help">
              Khóa này phải hoàn toàn giống với khóa chữ ký trong trang cấu hình MQTT Server, nếu không thông tin xác
              thực kết nối gửi đến thiết bị sẽ không qua được kiểm tra.
            </div>
          </el-form-item>
        </div>
      </el-card>

      <div class="environment-grid">
        <el-card class="config-card" shadow="never">
          <template #header>
            <div class="card-head">
              <div>
                <p class="card-kicker">Test</p>
                <h3>Phân phối môi trường kiểm thử</h3>
                <p class="card-description">
                  Dùng cho thiết bị phiên bản thử nghiệm hoặc xác minh trong mạng nội bộ. Khuyến nghị kiểm tra địa chỉ
                  có thể truy cập trước khi quyết định có phân phối đầu cuối MQTT đồng thời hay không.
                </p>
              </div>
              <div class="card-actions">
                <el-tag type="warning" effect="plain" round>Môi trường kiểm thử</el-tag>
                <el-button size="small" :loading="otaTestingTest" @click="testOtaEnv('test')"
                  >Kiểm tra kết nối</el-button
                >
              </div>
            </div>
          </template>

          <div class="section-stack">
            <section class="config-section">
              <div class="section-title">Phân phối WebSocket</div>
              <el-form-item label="WebSocket URL" prop="test.websocket.url">
                <el-input v-model="form.test.websocket.url" placeholder="Ví dụ: ws://host:port/milestones/v1/" />
              </el-form-item>
            </section>

            <section class="config-section">
              <div class="section-title">Phân phối MQTT</div>
              <el-form-item label="Trạng thái MQTT">
                <div class="switch-field">
                  <div>
                    <div class="switch-title">Ưu tiên phân phối đầu cuối MQTT</div>
                    <div class="field-help">
                      Firmware mặc định ưu tiên dùng MQTT; khi tắt, giá trị đầu cuối đã nhập vẫn được giữ lại để dễ bật
                      lại sau.
                    </div>
                  </div>
                  <el-switch v-model="form.test.mqtt.enable" />
                </div>
              </el-form-item>

              <el-form-item label="Đầu cuối MQTT" prop="test.mqtt.endpoint">
                <el-input
                  v-model="form.test.mqtt.endpoint"
                  :disabled="!form.test.mqtt.enable"
                  placeholder="Ví dụ: 127.0.0.1:1883"
                />
                <div class="field-help">
                  Cần xác nhận MQTT Server và UDP Server đều đã bật thì thiết bị mới ưu tiên dùng MQTT.
                </div>
              </el-form-item>
            </section>
          </div>
        </el-card>

        <el-card class="config-card" shadow="never">
          <template #header>
            <div class="card-head">
              <div>
                <p class="card-kicker">External</p>
                <h3>Phân phối môi trường bên ngoài</h3>
                <p class="card-description">
                  Dùng cho môi trường sản xuất hoặc mạng công khai. Khuyến nghị nhập địa chỉ WebSocket và MQTT có thể
                  truy cập thực tế, không nên dùng lại địa chỉ mạng nội bộ.
                </p>
              </div>
              <div class="card-actions">
                <el-tag type="success" effect="plain" round>Môi trường sản xuất</el-tag>
                <el-button size="small" :loading="otaTestingExternal" @click="testOtaEnv('external')"
                  >Kiểm tra kết nối</el-button
                >
              </div>
            </div>
          </template>

          <div class="section-stack">
            <section class="config-section">
              <div class="section-title">Phân phối WebSocket</div>
              <el-form-item label="WebSocket URL" prop="external.websocket.url">
                <el-input v-model="form.external.websocket.url" placeholder="Ví dụ: wss://example.com/milestones/v1/" />
              </el-form-item>
            </section>

            <section class="config-section">
              <div class="section-title">Phân phối MQTT</div>
              <el-form-item label="Trạng thái MQTT">
                <div class="switch-field">
                  <div>
                    <div class="switch-title">Phân phối MQTT trong môi trường sản xuất</div>
                    <div class="field-help">
                      Nếu môi trường sản xuất ưu tiên WebSocket, có thể tắt MQTT và chỉ giữ lại giá trị đầu cuối như dự
                      phòng.
                    </div>
                  </div>
                  <el-switch v-model="form.external.mqtt.enable" />
                </div>
              </el-form-item>

              <el-form-item label="Đầu cuối MQTT" prop="external.mqtt.endpoint">
                <el-input
                  v-model="form.external.mqtt.endpoint"
                  :disabled="!form.external.mqtt.enable"
                  placeholder="Ví dụ: broker.example.com:1883"
                />
              </el-form-item>
            </section>
          </div>
        </el-card>
      </div>

      <div class="footer-bar">
        <p class="footer-note">
          Sau khi lưu, cấu hình phân phối OTA mặc định sẽ được cập nhật; môi trường kiểm thử và môi trường bên ngoài có
          thể xác minh khả năng kết nối WebSocket và MQTT UDP riêng biệt.
        </p>
        <div class="footer-actions">
          <el-button plain :loading="loading" @click="loadConfig">Đặt lại về cấu hình hiện tại</el-button>
          <el-button type="primary" :loading="saving" @click="saveConfig">Lưu cấu hình</el-button>
        </div>
      </div>
    </el-form>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import api from '@/utils/api';

const loading = ref(false);
const saving = ref(false);
const otaTestingTest = ref(false);
const otaTestingExternal = ref(false);
const configId = ref(null);
const formRef = ref();

const createDefaultState = () => ({
  signature_key: 'milestones_ota_signature_key',
  test: {
    websocket: {
      url: 'ws://127.0.0.1:8989/milestones/v1/',
    },
    mqtt: {
      enable: true,
      endpoint: '127.0.0.1:1883',
    },
  },
  external: {
    websocket: {
      url: 'ws://127.0.0.1:8989/milestones/v1/',
    },
    mqtt: {
      enable: false,
      endpoint: '127.0.0.1:1883',
    },
  },
});

const form = reactive(createDefaultState());

const rules = {
  signature_key: [{ required: true, message: 'Vui lòng nhập khóa chữ ký', trigger: 'blur' }],
  'test.websocket.url': [
    {
      required: true,
      message: 'Vui lòng nhập WebSocket URL cho môi trường kiểm thử',
      trigger: 'blur',
    },
  ],
  'test.mqtt.endpoint': [
    {
      validator: (_, value, callback) => {
        if (form.test.mqtt.enable && !String(value || '').trim()) {
          callback(new Error('Đầu cuối MQTT không được để trống khi đã bật MQTT'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
  'external.websocket.url': [
    {
      required: true,
      message: 'Vui lòng nhập WebSocket URL cho môi trường bên ngoài',
      trigger: 'blur',
    },
  ],
  'external.mqtt.endpoint': [
    {
      validator: (_, value, callback) => {
        if (form.external.mqtt.enable && !String(value || '').trim()) {
          callback(new Error('Đầu cuối MQTT không được để trống khi đã bật MQTT'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
};

const applyState = (state) => {
  form.signature_key = state.signature_key;
  form.test.websocket.url = state.test.websocket.url;
  form.test.mqtt.enable = state.test.mqtt.enable;
  form.test.mqtt.endpoint = state.test.mqtt.endpoint;
  form.external.websocket.url = state.external.websocket.url;
  form.external.mqtt.enable = state.external.mqtt.enable;
  form.external.mqtt.endpoint = state.external.mqtt.endpoint;
};

const buildConfigObject = () => ({
  signature_key: String(form.signature_key || '').trim(),
  test: {
    websocket: {
      url: String(form.test.websocket.url || '').trim(),
    },
    mqtt: {
      enable: !!form.test.mqtt.enable,
      endpoint: String(form.test.mqtt.endpoint || '').trim(),
    },
  },
  external: {
    websocket: {
      url: String(form.external.websocket.url || '').trim(),
    },
    mqtt: {
      enable: !!form.external.mqtt.enable,
      endpoint: String(form.external.mqtt.endpoint || '').trim(),
    },
  },
});

const loadConfig = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/ota-configs');
    const configs = response.data?.data || [];

    if (configs.length > 0) {
      const config = configs[0];
      configId.value = config.id;

      try {
        const configData = JSON.parse(config.json_data || '{}');
        applyState({
          signature_key: configData.signature_key || 'milestones_ota_signature_key',
          test: {
            websocket: {
              url: configData.test?.websocket?.url || 'ws://127.0.0.1:8989/milestones/v1/',
            },
            mqtt: {
              enable: configData.test?.mqtt?.enable !== undefined ? configData.test.mqtt.enable : true,
              endpoint: configData.test?.mqtt?.endpoint || '127.0.0.1:1883',
            },
          },
          external: {
            websocket: {
              url: configData.external?.websocket?.url || 'ws://127.0.0.1:8989/milestones/v1/',
            },
            mqtt: {
              enable: configData.external?.mqtt?.enable !== undefined ? configData.external.mqtt.enable : false,
              endpoint: configData.external?.mqtt?.endpoint || '127.0.0.1:1883',
            },
          },
        });
      } catch (error) {
        ElMessage.warning('Định dạng cấu hình OTA bị lỗi, đã khôi phục về giá trị mặc định');
        applyState(createDefaultState());
      }
    } else {
      configId.value = null;
      applyState(createDefaultState());
    }
  } catch (error) {
    ElMessage.error('Tải cấu hình OTA thất bại');
  } finally {
    loading.value = false;
  }
};

const saveConfig = async () => {
  if (!formRef.value) return;

  try {
    await formRef.value.validate();
  } catch {
    return;
  }

  saving.value = true;
  try {
    const configData = {
      name: 'OTA配置',
      config_id: 'ota_ota_config',
      json_data: JSON.stringify(buildConfigObject()),
      enabled: true,
      is_default: true,
    };

    if (configId.value) {
      await api.put(`/admin/ota-configs/${configId.value}`, configData);
      ElMessage.success('Cấu hình OTA đã được cập nhật');
    } else {
      const response = await api.post('/admin/ota-configs', configData);
      configId.value = response.data?.data?.id || configId.value;
      ElMessage.success('Cấu hình OTA đã được lưu');
    }

    await loadConfig();
  } catch (error) {
    ElMessage.error(error.response?.data?.message || 'Lưu cấu hình OTA thất bại');
  } finally {
    saving.value = false;
  }
};

const testOtaEnv = async (env) => {
  const envConfig = env === 'test' ? form.test : form.external;
  const mqttEnabled = envConfig.mqtt.enable;
  const payload = buildConfigObject();
  const loadingRef = env === 'test' ? otaTestingTest : otaTestingExternal;

  loadingRef.value = true;
  try {
    const body = { types: ['ota'], data: { ota: { ota_ota_config: payload } } };
    const res = await api.post('/admin/configs/test', body, { timeout: 30000 });
    const data = res.data?.data ?? res.data;
    const otaResult = data?.ota?.ota_ota_config;
    const label = env === 'test' ? 'Môi trường kiểm thử' : 'Môi trường bên ngoài';

    if (!otaResult) {
      ElMessage.error(`${label}: Không nhận được kết quả kiểm tra`);
      return;
    }

    const wsResult = otaResult.websocket || {};
    const wsOk = wsResult.ok || false;
    const wsMsg = wsResult.message || 'Kiểm tra WebSocket thất bại';
    const wsMs = wsResult.first_packet_ms;

    const mqttResult = otaResult.mqtt_udp;
    let mqttOk = true;
    let mqttMsg = '';
    let mqttMs = 0;

    if (mqttEnabled && mqttResult) {
      mqttOk = mqttResult.ok || false;
      mqttMsg = mqttResult.message || 'Kiểm tra MQTT UDP thất bại';
      mqttMs = mqttResult.first_packet_ms || 0;
    } else if (mqttEnabled) {
      mqttOk = false;
      mqttMsg = 'MQTT UDP không trả về kết quả';
    }

    let message = wsOk ? `WebSocket: ${wsMsg}` : `WebSocket: ${wsMsg}`;
    if (wsMs != null) message += ` (${wsMs}ms)`;

    if (mqttEnabled) {
      message += ' | ';
      message += `MQTT UDP: ${mqttMsg}`;
      if (mqttMs != null) message += ` (${mqttMs}ms)`;
    }

    if (wsOk && (!mqttEnabled || mqttOk)) {
      ElMessage.success(`${label}: ${message}`);
    } else {
      ElMessage.warning(`${label}: ${message}`);
    }
  } catch (error) {
    ElMessage.error(error.response?.data?.error || 'Yêu cầu kiểm tra thất bại');
  } finally {
    loadingRef.value = false;
  }
};

onMounted(() => {
  loadConfig();
});
</script>

<style scoped>
.ota-config {
  padding: 0 24px 32px;
}

.config-form {
  display: grid;
  gap: 24px;
}

.environment-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
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

.card-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
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
.footer-note {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--apple-text-secondary);
}

.field-grid {
  display: grid;
  gap: 20px 18px;
}

.field-span-full {
  grid-column: 1 / -1;
}

.section-stack {
  display: grid;
  gap: 24px;
}

.config-section {
  display: grid;
  gap: 18px;
}

.config-section + .config-section {
  padding-top: 24px;
  border-top: 1px solid rgba(229, 229, 234, 0.72);
}

.section-title {
  font-size: 15px;
  font-weight: 700;
  color: var(--apple-text);
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

.footer-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 0 4px;
}

.footer-note {
  max-width: 700px;
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
  .environment-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .ota-config {
    padding: 0 16px 24px;
  }

  :deep(.el-card__body) {
    padding: 20px;
  }

  :deep(.el-card__header) {
    padding: 20px 20px 0;
  }

  .card-head,
  .footer-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .footer-actions {
    justify-content: stretch;
  }

  .footer-actions :deep(.el-button) {
    flex: 1;
  }
}
</style>
