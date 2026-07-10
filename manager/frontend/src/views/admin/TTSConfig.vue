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
      <el-table-column prop="enabled" label="Trạng thái" width="80" align="center">
        <template #default="scope">
          <el-switch v-model="scope.row.enabled" @change="toggleEnable(scope.row)" />
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="Mặc định" width="80" align="center">
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
      <el-table-column prop="created_at" label="Ngày tạo" width="180">
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
            @click="testConfig(scope.row, 'tts')"
          >
            Kiểm tra
          </el-button>
          <el-button size="small" type="danger" @click="deleteConfig(scope.row.id)">Xóa</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Dialog thêm/chỉnh sửa cấu hình -->
    <el-dialog
      v-model="showDialog"
      :title="editingConfig ? 'Chỉnh sửa cấu hình TTS' : 'Thêm cấu hình TTS'"
      width="720px"
      @close="handleDialogClose"
    >
      <TTSConfigForm
        ref="formRef"
        :model="form"
        :rules="rules"
        :voice-options="voiceOptions"
        :voice-loading="voiceLoading"
        @request-voice-options="handleVoiceOptionsRequest"
      />

      <template #footer>
        <el-button @click="handleDialogClose">Hủy</el-button>
        <el-button type="warning" plain @click="testCurrentConfig" :loading="testingCurrent">Kiểm tra</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">Lưu</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed, watch, nextTick } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import api from '../../utils/api';
import { testSingleConfig, testWithData, parseJsonData } from '../../utils/configTest';
import TTSConfigForm from './forms/TTSConfigForm.vue';
import { TTS_PROVIDERS_WITH_VOICES } from './forms/ttsProviderOptions';
import { resolveTTSProvider } from './forms/configProviderUtils';

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

// Danh sách giọng nói
const voiceOptions = ref([]);
const voiceLoading = ref(false);

const form = reactive({
  name: '',
  config_id: '',
  provider: 'doubao_ws',
  is_default: false,
  enabled: true,
  double_stream: false,
  cosyvoice: {
    api_url: 'https://tts.linkerai.cn/tts',
    spk_id: 'spk_id',
    frame_duration: 60,
    target_sr: 24000,
    audio_format: 'mp3',
    instruct_text: '你好',
  },
  qwen_tts: {
    api_key: '',
    api_url: 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation',
    region: 'beijing',
    model: 'qwen3-tts-flash',
    voice: 'Cherry',
    language_type: 'Chinese',
    stream: true,
    frame_duration: 60,
  },
  doubao: {
    appid: '6886011847',
    access_token: 'access_token',
    model: 'seed-tts-2.0-standard',
    voice: 'BV001_streaming',
    api_url: 'https://openspeech.bytedance.com/api/v3/tts/unidirectional',
  },
  doubao_ws: {
    appid: '6886011847',
    access_token: 'access_token',
    model: 'seed-tts-2.0-standard',
    resource_id: '',
    voice: '',
    ws_url: 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream',
  },
  edge: {
    voice: 'zh-CN-XiaoxiaoNeural',
    rate: '+0%',
    volume: '+0%',
    pitch: '+0Hz',
    connect_timeout: 10,
    receive_timeout: 60,
  },
  edge_offline: {
    server_url: 'ws://localhost:8080/tts',
    timeout: 30,
    sample_rate: 16000,
    channels: 1,
    frame_duration: 20,
  },
  openai: {
    api_key: '',
    api_url: 'https://api.openai.com/v1/audio/speech',
    model: 'tts-1',
    voice: 'alloy',
    response_format: 'mp3',
    speed: 1.0,
    stream: true,
    frame_duration: 60,
  },
  xunfei: {
    app_id: '',
    api_key: '',
    api_secret: '',
    ws_url: 'wss://tts-api.xfyun.cn/v2/tts',
    voice: 'xiaoyan',
    audio_encoding: 'raw',
    sample_rate: 16000,
    speed: 50,
    volume: 50,
    pitch: 50,
    tte: 'UTF8',
    reg: 0,
    rdn: 0,
    frame_duration: 60,
    connect_timeout: 10,
    read_timeout: 30,
  },
  xunfei_super_tts: {
    app_id: '',
    api_key: '',
    api_secret: '',
    ws_url: 'wss://cbm01.cn-huabei-1.xf-yun.com/v1/private/mcd9m97e6',
    voice: 'x6_lingxiaoxue_pro',
    audio_encoding: 'raw',
    sample_rate: 24000,
    speed: 50,
    volume: 50,
    pitch: 50,
    bgs: 0,
    reg: 0,
    rdn: 0,
    rhy: 0,
    oral_level: 'mid',
    spark_assist: 1,
    stop_split: 0,
    remain: 0,
    frame_duration: 60,
    connect_timeout: 10,
    read_timeout: 30,
  },
  indextts_vllm: {
    api_url: 'http://127.0.0.1:7860',
    api_key: '',
    model: 'indextts-vllm',
    voice: '',
    frame_duration: 60,
  },
  zhipu: {
    api_key: '',
    api_url: 'https://open.bigmodel.cn/api/paas/v4/audio/speech',
    model: 'glm-tts',
    voice: 'tongtong',
    response_format: 'pcm',
    speed: 1.0,
    volume: 1.0,
    stream: true,
    encode_format: 'base64',
    frame_duration: 60,
  },
  minimax: {
    api_key: '',
    model: 'speech-2.8-hd',
    voice: 'male-qn-qingse',
    speed: 1.0,
    vol: 1.0,
    pitch: 0,
    sample_rate: 32000,
    bitrate: 128000,
    format: 'mp3',
    channel: 1,
  },
});

const rules = {
  name: [{ required: true, message: 'Vui lòng nhập tên cấu hình', trigger: 'blur' }],
  config_id: [{ required: true, message: 'Vui lòng nhập ID cấu hình', trigger: 'blur' }],
  provider: [{ required: true, message: 'Vui lòng chọn nhà cung cấp', trigger: 'change' }],
  // Quy tắc CosyVoice
  'cosyvoice.api_url': [{ required: true, message: 'Vui lòng nhập API URL', trigger: 'blur' }],
  'cosyvoice.spk_id': [{ required: true, message: 'Vui lòng nhập ID người nói', trigger: 'blur' }],
  // Quy tắc Doubao TTS
  'doubao.appid': [{ required: true, message: 'Vui lòng nhập App ID', trigger: 'blur' }],
  'doubao.access_token': [{ required: true, message: 'Vui lòng nhập Access Token', trigger: 'blur' }],
  'doubao.model': [{ required: true, message: 'Vui lòng chọn mô hình', trigger: 'change' }],
  'doubao.voice': [{ required: true, message: 'Vui lòng nhập giọng nói', trigger: 'blur' }],
  'doubao.api_url': [{ required: true, message: 'Vui lòng nhập API URL', trigger: 'blur' }],
  // Quy tắc Doubao WebSocket
  'doubao_ws.appid': [{ required: true, message: 'Vui lòng nhập App ID', trigger: 'blur' }],
  'doubao_ws.access_token': [{ required: true, message: 'Vui lòng nhập Access Token', trigger: 'blur' }],
  'doubao_ws.model': [{ required: true, message: 'Vui lòng chọn mô hình', trigger: 'change' }],
  'doubao_ws.voice': [{ required: true, message: 'Vui lòng nhập giọng nói', trigger: 'blur' }],
  'doubao_ws.ws_url': [{ required: true, message: 'Vui lòng nhập WebSocket URL', trigger: 'blur' }],
  // Quy tắc Edge TTS
  'edge.voice': [{ required: true, message: 'Vui lòng nhập giọng nói', trigger: 'blur' }],
  'edge.rate': [{ required: true, message: 'Vui lòng nhập tốc độ đọc', trigger: 'blur' }],
  'edge.volume': [{ required: true, message: 'Vui lòng nhập âm lượng', trigger: 'blur' }],
  // Quy tắc Edge Offline
  'edge_offline.server_url': [{ required: true, message: 'Vui lòng nhập URL máy chủ', trigger: 'blur' }],
  // Quy tắc OpenAI TTS
  'openai.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
  // Quy tắc Xunfei TTS
  'xunfei.app_id': [{ required: true, message: 'Vui lòng nhập App ID', trigger: 'blur' }],
  'xunfei.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
  'xunfei.api_secret': [{ required: true, message: 'Vui lòng nhập API Secret', trigger: 'blur' }],
  'xunfei.ws_url': [{ required: true, message: 'Vui lòng nhập WebSocket URL', trigger: 'blur' }],
  'xunfei.voice': [{ required: true, message: 'Vui lòng nhập giọng nói', trigger: 'blur' }],
  'xunfei_super_tts.app_id': [{ required: true, message: 'Vui lòng nhập App ID', trigger: 'blur' }],
  'xunfei_super_tts.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
  'xunfei_super_tts.api_secret': [{ required: true, message: 'Vui lòng nhập API Secret', trigger: 'blur' }],
  'xunfei_super_tts.ws_url': [{ required: true, message: 'Vui lòng nhập WebSocket URL', trigger: 'blur' }],
  'xunfei_super_tts.voice': [{ required: true, message: 'Vui lòng nhập giọng nói', trigger: 'blur' }],
  // Quy tắc Zhipu TTS
  'zhipu.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
  // Quy tắc Minimax TTS
  'minimax.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
  // Quy tắc Qwen TTS
  'qwen_tts.api_key': [{ required: true, message: 'Vui lòng nhập API Key', trigger: 'blur' }],
  'indextts_vllm.api_url': [{ required: true, message: 'Vui lòng nhập API URL', trigger: 'blur' }],
};

const loadConfigs = async () => {
  loading.value = true;
  try {
    const response = await api.get('/admin/tts-configs');
    configs.value = (response.data.data || []).map(normalizeTTSConfigRow);
  } catch (error) {
    ElMessage.error('Tải cấu hình thất bại');
  } finally {
    loading.value = false;
  }
};

function normalizeTTSConfigRow(row) {
  const data = parseJsonData(row?.json_data);
  return {
    ...row,
    provider: resolveTTSProvider(row?.provider, row?.config_id, data),
  };
}

const editConfig = (config) => {
  config = normalizeTTSConfigRow(config);
  editingConfig.value = config;
  form.name = config.name;
  form.config_id = config.config_id;
  form.provider = config.provider;
  form.is_default = config.is_default;
  form.enabled = config.enabled;
  form.double_stream = false;

  // IndexTTS: chỉ tải danh sách giọng khi mở dropdown
  loadVoiceOptions(config.provider);

  // Phân tích JSON cấu hình và điền vào các trường tương ứng
  try {
    const configData = JSON.parse(config.json_data || '{}');
    form.double_stream = configData.double_stream === true;

    switch (config.provider) {
      case 'cosyvoice':
        form.cosyvoice.api_url = configData.api_url || '';
        form.cosyvoice.spk_id = configData.spk_id || '';
        form.cosyvoice.frame_duration = configData.frame_duration || 60;
        form.cosyvoice.target_sr = configData.target_sr || 24000;
        form.cosyvoice.audio_format = configData.audio_format || 'mp3';
        form.cosyvoice.instruct_text = configData.instruct_text || '';
        break;
      case 'doubao':
        form.doubao.appid = configData.appid || '';
        form.doubao.access_token = configData.access_token || '';
        form.doubao.model = configData.model || 'seed-tts-2.0-standard';
        form.doubao.voice = configData.voice || '';
        form.doubao.api_url = configData.api_url || 'https://openspeech.bytedance.com/api/v3/tts/unidirectional';
        break;
      case 'doubao_ws':
        form.doubao_ws.appid = configData.appid || '';
        form.doubao_ws.access_token = configData.access_token || '';
        form.doubao_ws.model = configData.model || 'seed-tts-2.0-standard';
        form.doubao_ws.resource_id = configData.resource_id || '';
        form.doubao_ws.voice = configData.voice || '';
        form.doubao_ws.ws_url =
          configData.ws_url ||
          (configData.ws_host
            ? `wss://${configData.ws_host}/api/v3/tts/unidirectional/stream`
            : 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream');
        break;
      case 'edge':
        form.edge.voice = configData.voice || '';
        form.edge.rate = configData.rate || '+0%';
        form.edge.volume = configData.volume || '+0%';
        form.edge.pitch = configData.pitch || '+0Hz';
        form.edge.connect_timeout = configData.connect_timeout || 10;
        form.edge.receive_timeout = configData.receive_timeout || 60;
        break;
      case 'edge_offline':
        form.edge_offline.server_url = configData.server_url || '';
        form.edge_offline.timeout = configData.timeout || 30;
        form.edge_offline.sample_rate = configData.sample_rate || 16000;
        form.edge_offline.channels = configData.channels || 1;
        form.edge_offline.frame_duration = configData.frame_duration || 20;
        break;
      case 'aliyun_qwen':
        form.qwen_tts.api_key = configData.api_key || '';
        form.qwen_tts.api_url =
          configData.api_url || 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation';
        form.qwen_tts.region = configData.region || 'beijing';
        form.qwen_tts.model = configData.model || 'qwen3-tts-flash';
        form.qwen_tts.voice = configData.voice || 'Cherry';
        form.qwen_tts.language_type = configData.language_type || 'Chinese';
        form.qwen_tts.stream = configData.stream !== undefined ? configData.stream : true;
        form.qwen_tts.frame_duration = configData.frame_duration || 60;
        break;
      case 'openai':
        form.openai.api_key = configData.api_key || '';
        form.openai.api_url = configData.api_url || 'https://api.openai.com/v1/audio/speech';
        form.openai.model = configData.model || 'tts-1';
        form.openai.voice = configData.voice || 'alloy';
        form.openai.response_format = configData.response_format || 'mp3';
        form.openai.speed = configData.speed || 1.0;
        form.openai.stream = configData.stream !== undefined ? configData.stream : true;
        form.openai.frame_duration = configData.frame_duration || 60;
        break;
      case 'xunfei':
        form.xunfei.app_id = configData.app_id || '';
        form.xunfei.api_key = configData.api_key || '';
        form.xunfei.api_secret = configData.api_secret || '';
        form.xunfei.ws_url = configData.ws_url || 'wss://tts-api.xfyun.cn/v2/tts';
        form.xunfei.voice = configData.voice || 'xiaoyan';
        form.xunfei.audio_encoding = configData.audio_encoding || 'raw';
        form.xunfei.sample_rate = configData.sample_rate || 16000;
        form.xunfei.speed = configData.speed ?? 50;
        form.xunfei.volume = configData.volume ?? 50;
        form.xunfei.pitch = configData.pitch ?? 50;
        form.xunfei.tte = configData.tte || 'UTF8';
        form.xunfei.reg = configData.reg ?? 0;
        form.xunfei.rdn = configData.rdn ?? 0;
        form.xunfei.frame_duration = configData.frame_duration || 60;
        form.xunfei.connect_timeout = configData.connect_timeout || 10;
        form.xunfei.read_timeout = configData.read_timeout || 30;
        break;
      case 'xunfei_super_tts':
        form.xunfei_super_tts.app_id = configData.app_id || '';
        form.xunfei_super_tts.api_key = configData.api_key || '';
        form.xunfei_super_tts.api_secret = configData.api_secret || '';
        form.xunfei_super_tts.ws_url = configData.ws_url || 'wss://cbm01.cn-huabei-1.xf-yun.com/v1/private/mcd9m97e6';
        form.xunfei_super_tts.voice = configData.voice || 'x6_lingxiaoxue_pro';
        form.xunfei_super_tts.audio_encoding = configData.audio_encoding || 'raw';
        form.xunfei_super_tts.sample_rate = configData.sample_rate || 24000;
        form.xunfei_super_tts.speed = configData.speed ?? 50;
        form.xunfei_super_tts.volume = configData.volume ?? 50;
        form.xunfei_super_tts.pitch = configData.pitch ?? 50;
        form.xunfei_super_tts.bgs = configData.bgs ?? 0;
        form.xunfei_super_tts.reg = configData.reg ?? 0;
        form.xunfei_super_tts.rdn = configData.rdn ?? 0;
        form.xunfei_super_tts.rhy = configData.rhy ?? 0;
        form.xunfei_super_tts.oral_level = configData.oral_level || 'mid';
        form.xunfei_super_tts.spark_assist = configData.spark_assist ?? 1;
        form.xunfei_super_tts.stop_split = configData.stop_split ?? 0;
        form.xunfei_super_tts.remain = configData.remain ?? 0;
        form.xunfei_super_tts.frame_duration = configData.frame_duration || 60;
        form.xunfei_super_tts.connect_timeout = configData.connect_timeout || 10;
        form.xunfei_super_tts.read_timeout = configData.read_timeout || 30;
        break;
      case 'indextts_vllm':
        form.indextts_vllm.api_url = configData.api_url || 'http://127.0.0.1:7860';
        form.indextts_vllm.api_key = configData.api_key || '';
        form.indextts_vllm.model = configData.model || 'indextts-vllm';
        form.indextts_vllm.voice = configData.voice || '';
        form.indextts_vllm.frame_duration = configData.frame_duration || 60;
        break;
      case 'zhipu':
        // Đọc cấu hình Zhipu từ json_data
        form.zhipu.api_key = configData.api_key || '';
        form.zhipu.api_url = configData.api_url || 'https://open.bigmodel.cn/api/paas/v4/audio/speech';
        form.zhipu.model = configData.model || 'glm-tts';
        form.zhipu.voice = configData.voice || 'tongtong';
        form.zhipu.response_format = configData.response_format || 'pcm';
        form.zhipu.speed = configData.speed || 1.0;
        form.zhipu.volume = configData.volume || 1.0;
        form.zhipu.stream = configData.stream !== undefined ? configData.stream : true;
        form.zhipu.encode_format = configData.encode_format || 'base64';
        form.zhipu.frame_duration = configData.frame_duration || 60;
        break;
      case 'minimax':
        form.minimax.api_key = configData.api_key || '';
        form.minimax.model = configData.model || 'speech-2.8-hd';
        form.minimax.voice = configData.voice || 'male-qn-qingse';
        form.minimax.speed = configData.speed || 1.0;
        form.minimax.vol = configData.vol || configData.volume || 1.0;
        form.minimax.pitch = configData.pitch || 0;
        form.minimax.sample_rate = configData.sample_rate || 32000;
        form.minimax.bitrate = configData.bitrate || 128000;
        form.minimax.format = configData.format || 'mp3';
        form.minimax.channel = configData.channel || 1;
        break;
    }
  } catch (error) {
    console.error('Phân tích JSON cấu hình thất bại:', error);
  }

  showDialog.value = true;
};

const handleSave = async () => {
  if (!formRef.value) return;

  await formRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true;
      try {
        // Nếu là cấu hình mới và chưa có cấu hình nào, tự động đặt làm mặc định
        const isFirstConfig = !editingConfig.value && configs.value.length === 0;

        const configData = {
          name: form.name,
          config_id: form.config_id,
          provider: form.provider,
          is_default: isFirstConfig || form.is_default, // Tự động đặt mặc định khi thêm lần đầu
          enabled: form.enabled !== undefined ? form.enabled : true,
          json_data: formRef.value.getJsonData(),
        };

        if (editingConfig.value) {
          await api.put(`/admin/tts-configs/${editingConfig.value.id}`, configData);
          ElMessage.success('Cập nhật cấu hình thành công');
        } else {
          await api.post('/admin/tts-configs', configData);
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
    ElMessage.success(`${config.enabled ? 'Bật' : 'Tắt'} thành công`);
  } catch (error) {
    // Khôi phục trạng thái switch
    config.enabled = !config.enabled;
    ElMessage.error('Thao tác thất bại');
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

    await api.put(`/admin/tts-configs/${config.id}`, configData);
    ElMessage.success(config.is_default ? 'Đặt mặc định thành công' : 'Bỏ mặc định thành công');

    // Tải lại danh sách để cập nhật trạng thái mặc định của các cấu hình khác
    loadConfigs();
  } catch (error) {
    // Khôi phục trạng thái switch
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
    ElMessage.warning('Không có cấu hình nào đang được bật');
    return;
  }
  testingAll.value = true;
  testResults.value = {};
  let okCount = 0;
  try {
    for (const row of list) {
      try {
        const result = await testSingleConfig('tts', row.config_id);
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
    const result = await testWithData('tts', { [configId]: payload });
    if (result.ok) {
      ElMessage.success(formatTestMessage(result) || 'Kiểm tra thành công');
    } else {
      ElMessage.warning(result.message || 'Kiểm tra không đạt');
    }
  } catch (err) {
    ElMessage.error(err.response?.data?.error || 'Yêu cầu kiểm tra thất bại');
  } finally {
    testingCurrent.value = false;
  }
};

const deleteConfig = async (id) => {
  try {
    await ElMessageBox.confirm('Bạn có chắc chắn muốn xóa cấu hình này không?', 'Xác nhận', {
      confirmButtonText: 'Xác nhận',
      cancelButtonText: 'Hủy',
      type: 'warning',
    });

    await api.delete(`/admin/tts-configs/${id}`);
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
  Object.assign(form, {
    name: '',
    config_id: '',
    provider: 'doubao_ws',
    is_default: false,
    enabled: true,
    cosyvoice: {
      api_url: 'https://tts.linkerai.top/tts',
      spk_id: 'spk_id',
      frame_duration: 60,
      target_sr: 24000,
      audio_format: 'mp3',
      instruct_text: '你好',
    },
    qwen_tts: {
      api_key: '',
      api_url: 'https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation',
      region: 'beijing',
      model: 'qwen3-tts-flash',
      voice: 'Cherry',
      language_type: 'Chinese',
      stream: true,
      frame_duration: 60,
    },
    doubao: {
      appid: '6886011847',
      access_token: 'access_token',
      model: 'seed-tts-2.0-standard',
      voice: 'BV001_streaming',
      api_url: 'https://openspeech.bytedance.com/api/v3/tts/unidirectional',
    },
    doubao_ws: {
      appid: '6886011847',
      access_token: 'access_token',
      model: 'seed-tts-2.0-standard',
      resource_id: '',
      voice: '',
      ws_url: 'wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream',
    },
    edge: {
      voice: 'zh-CN-XiaoxiaoNeural',
      rate: '+0%',
      volume: '+0%',
      pitch: '+0Hz',
      connect_timeout: 10,
      receive_timeout: 60,
    },
    edge_offline: {
      server_url: 'ws://localhost:8080/tts',
      timeout: 30,
      sample_rate: 16000,
      channels: 1,
      frame_duration: 20,
    },
    openai: {
      api_key: '',
      api_url: 'https://api.openai.com/v1/audio/speech',
      model: 'tts-1',
      voice: 'alloy',
      response_format: 'mp3',
      speed: 1.0,
      stream: true,
      frame_duration: 60,
    },
    xunfei: {
      app_id: '',
      api_key: '',
      api_secret: '',
      ws_url: 'wss://tts-api.xfyun.cn/v2/tts',
      voice: 'xiaoyan',
      audio_encoding: 'raw',
      sample_rate: 16000,
      speed: 50,
      volume: 50,
      pitch: 50,
      tte: 'UTF8',
      reg: 0,
      rdn: 0,
      frame_duration: 60,
      connect_timeout: 10,
      read_timeout: 30,
    },
    xunfei_super_tts: {
      app_id: '',
      api_key: '',
      api_secret: '',
      ws_url: 'wss://cbm01.cn-huabei-1.xf-yun.com/v1/private/mcd9m97e6',
      voice: 'x6_lingxiaoxue_pro',
      audio_encoding: 'raw',
      sample_rate: 24000,
      speed: 50,
      volume: 50,
      pitch: 50,
      bgs: 0,
      reg: 0,
      rdn: 0,
      rhy: 0,
      oral_level: 'mid',
      spark_assist: 1,
      stop_split: 0,
      remain: 0,
      frame_duration: 60,
      connect_timeout: 10,
      read_timeout: 30,
    },
    indextts_vllm: {
      api_url: 'http://127.0.0.1:7860',
      api_key: '',
      model: 'indextts-vllm',
      voice: '',
      frame_duration: 60,
    },
    zhipu: {
      api_key: '',
      api_url: 'https://open.bigmodel.cn/api/paas/v4/audio/speech',
      model: 'glm-tts',
      voice: 'tongtong',
      response_format: 'pcm',
      speed: 1.0,
      volume: 1.0,
      stream: true,
      frame_duration: 60,
    },
    minimax: {
      api_key: '',
      model: 'speech-2.8-hd',
      voice: 'male-qn-qingse',
      speed: 1.0,
      vol: 1.0,
      pitch: 0,
      sample_rate: 32000,
      bitrate: 128000,
      format: 'mp3',
      channel: 1,
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
  return new Date(dateString).toLocaleString('vi-VN');
};

// Tải danh sách giọng nói
const loadVoiceOptions = async (provider, options = {}) => {
  const trigger = options?.trigger || 'auto';
  if (!provider) {
    voiceOptions.value = [];
    return;
  }

  // IndexTTS chỉ tải khi mở dropdown
  if (provider === 'indextts_vllm' && trigger !== 'dropdown') {
    voiceOptions.value = [];
    return;
  }

  // Chỉ những provider này mới cần lấy danh sách giọng từ backend
  if (!TTS_PROVIDERS_WITH_VOICES.includes(provider)) {
    voiceOptions.value = [];
    return;
  }

  voiceLoading.value = true;
  try {
    const params = { provider, config_id: form.config_id || undefined };
    if (provider === 'indextts_vllm') {
      const apiURL = String(form.indextts_vllm?.api_url || '').trim();
      const apiKey = String(form.indextts_vllm?.api_key || '').trim();
      if (apiURL) {
        params.api_url = apiURL;
      }
      if (apiKey) {
        params.api_key = apiKey;
      }
    }
    const response = await api.get(`/user/voice-options`, {
      params,
    });
    voiceOptions.value = response.data.data || [];
  } catch (error) {
    console.error('Tải danh sách giọng nói thất bại:', error);
    voiceOptions.value = [];
  } finally {
    voiceLoading.value = false;
  }
};

const handleVoiceOptionsRequest = (provider) => {
  if (!showDialog.value) return;
  loadVoiceOptions(provider || form.provider, { trigger: 'dropdown' });
};

// Theo dõi thay đổi provider, tự động tải danh sách giọng tương ứng
watch(
  () => form.provider,
  (newProvider) => {
    if (showDialog.value) {
      loadVoiceOptions(newProvider);
    }
  },
  { immediate: false },
);

// Theo dõi khi dialog mở, tải danh sách giọng của provider hiện tại (nextTick đảm bảo dialog đã render xong)
watch(showDialog, (isOpen) => {
  if (isOpen && form.provider) {
    nextTick(() => loadVoiceOptions(form.provider));
  }
});

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
