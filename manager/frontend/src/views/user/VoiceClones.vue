<template>
  <div class="voice-clones-page">
    <div class="page-actions">
      <el-button type="primary" @click="openCreateDialog">Tạo giọng đọc sao chép</el-button>
    </div>

    <el-table :data="voiceClones" v-loading="loading" stripe style="width: 100%" table-layout="fixed">
      <el-table-column prop="name" label="Tên" min-width="120" show-overflow-tooltip />
      <el-table-column prop="provider" label="Nhà cung cấp" width="100" show-overflow-tooltip />
      <el-table-column label="Cấu hình TTS" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">{{ `${row.tts_config_name || '-'} (${row.tts_config_id || '-'})` }}</template>
      </el-table-column>
      <el-table-column prop="provider_voice_id" label="ID giọng sao chép" min-width="160" show-overflow-tooltip />
      <el-table-column v-if="authStore.isAdmin" label="Chia sẻ cho tất cả" width="140" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="!!row.shared_to_all"
            :disabled="normalizeCloneStatus(row) !== 'active' || shareSubmittingID === row.id"
            @change="(val) => toggleSharedToAll(row, val)"
          />
        </template>
      </el-table-column>
      <el-table-column label="Trạng thái tác vụ" width="100">
        <template #default="{ row }">
          <el-tag :type="getCloneStatusTagType(row)" size="small">{{ formatCloneStatus(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="Nguyên nhân thất bại" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ getCloneLastError(row) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="Thời gian tạo" width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="Thao tác" width="460">
        <template #default="{ row }">
          <div class="action-buttons">
            <el-button
              size="small"
              type="primary"
              plain
              :loading="previewUploadSubmittingID === row.id"
              @click="previewUploadedAudio(row)"
            >
              Âm thanh gốc
            </el-button>
            <el-button
              v-if="canPreviewClonedVoice(row)"
              size="small"
              type="success"
              :loading="previewClonedSubmittingID === row.id"
              @click="previewClonedVoice(row)"
            >
              Nghe thử sao chép
            </el-button>
            <el-button size="small" type="primary" plain @click="openEditDialog(row)">Chỉnh sửa</el-button>
            <el-button
              v-if="canRetryClone(row)"
              size="small"
              type="warning"
              plain
              :loading="retrySubmittingID === row.id"
              @click="retryClone(row)"
            >
              Sao chép lại
            </el-button>
            <el-button
              v-if="canAppendRefAudio(row)"
              size="small"
              type="primary"
              plain
              :loading="appendAudioSubmittingID === row.id"
              @click="openAppendAudioDialog(row)"
            >
              Thêm âm thanh tham chiếu
            </el-button>
            <el-button
              size="small"
              type="danger"
              plain
              :loading="deleteSubmittingID === row.id"
              @click="deleteClone(row)"
            >
              Xóa
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <input
      ref="appendAudioInputRef"
      type="file"
      :accept="uploadAcceptTypes"
      style="display: none"
      @change="handleAppendAudioFileChange"
    />

    <el-dialog v-model="createDialogVisible" title="Tạo giọng đọc sao chép" width="720px">
      <el-form label-width="140px">
        <el-form-item label="Tên sao chép">
          <el-input v-model="form.name" placeholder="Tuỳ chọn, không điền sẽ tự dùng tên tệp" />
        </el-form-item>
        <el-form-item label="Cấu hình TTS" required>
          <el-select
            v-model="form.tts_config_id"
            placeholder="Chọn cấu hình TTS có thể sao chép"
            style="width: 100%"
            @change="onConfigChange"
          >
            <el-option
              v-for="cfg in cloneEnabledConfigs"
              :key="cfg.config_id"
              :label="`${cfg.name} (${cfg.config_id})`"
              :value="cfg.config_id"
            />
          </el-select>
          <div v-if="isAliyunQwenProvider" class="help">
            Lưu ý: Sau khi chọn giọng sao chép này, hệ thống sẽ tự chuyển sang mô hình {{ qwenCloneRuntimeModel }}
          </div>
          <el-alert
            v-if="createChargeNotice.message"
            class="clone-charge-alert"
            :title="createChargeNotice.message"
            :type="createChargeNotice.type"
            :closable="false"
            show-icon
          />
        </el-form-item>
        <el-form-item label="Nguồn âm thanh">
          <el-radio-group v-model="form.source_type">
            <el-radio label="upload">Tải lên âm thanh</el-radio>
            <el-radio label="record">Ghi âm trình duyệt</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="form.source_type === 'upload'" label="Tệp âm thanh" required>
          <input type="file" :accept="uploadAcceptTypes" @change="handleFileChange" />
          <div class="help">{{ audioRequirementText }}</div>
        </el-form-item>

        <el-form-item v-else label="Ghi âm trình duyệt" required>
          <el-button :disabled="isRecording" @click="startRecording">Bắt đầu ghi âm</el-button>
          <el-button :disabled="!isRecording" type="warning" @click="stopRecording">Dừng ghi âm</el-button>
          <audio
            v-if="recordPreviewUrl"
            :src="recordPreviewUrl"
            controls
            style="display: block; width: 100%; margin-top: 10px"
          />
          <div class="help">{{ audioRequirementText }}</div>
        </el-form-item>

        <el-form-item
          :label="capability.requires_transcript ? 'Văn bản tương ứng âm thanh *' : 'Văn bản tương ứng âm thanh'"
        >
          <el-input
            v-model="form.transcript"
            type="textarea"
            :rows="4"
            :placeholder="
              capability.requires_transcript
                ? 'Nhà cung cấp yêu cầu nhập văn bản âm thanh'
                : 'Tuỳ chọn, không điền cũng có thể gửi'
            "
          />
          <div class="help">
            Yêu cầu: {{ capability.min_text_len || 0 }} - {{ capability.max_text_len || 4000 }} ký tự
          </div>
        </el-form-item>

        <el-form-item label="Ngôn ngữ văn bản">
          <el-select v-model="form.transcript_lang" style="width: 220px">
            <el-option label="Tiếng Việt (vi-VN)" value="vi-VN" />
            <el-option label="Tiếng Anh (en-US)" value="en-US" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">Hủy</el-button>
        <el-button type="primary" :loading="submitting" @click="submitClone">Gửi sao chép</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="audioDialogVisible" title="Âm thanh gốc sao chép" width="720px">
      <el-table :data="currentAudios" stripe>
        <el-table-column prop="source_type" label="Nguồn" width="90" />
        <el-table-column prop="file_name" label="Tên tệp" min-width="220" />
        <el-table-column prop="transcript" label="Văn bản tương ứng" min-width="240" show-overflow-tooltip />
        <el-table-column label="Phát" width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="playAudio(row)">Phát</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" title="Chỉnh sửa giọng đọc sao chép" width="620px" @close="resetEditForm">
      <el-form label-width="120px">
        <el-form-item label="Tên">
          <el-input v-model="editForm.name" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="Nhà cung cấp">
          <el-input v-model="editForm.provider" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="Cấu hình TTS">
          <el-input v-model="editForm.ttsConfigDisplay" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="ID giọng sao chép">
          <el-input v-model="editForm.providerVoiceID" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="Trạng thái tác vụ">
          <el-input v-model="editForm.statusText" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item label="Thời gian tạo">
          <el-input v-model="editForm.createdAtText" readonly class="readonly-field" />
        </el-form-item>
        <el-form-item v-if="editForm.lastError" label="Nguyên nhân thất bại">
          <el-input v-model="editForm.lastError" type="textarea" :rows="3" readonly class="readonly-field" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">Hủy</el-button>
        <el-button type="primary" :loading="editSubmitting" @click="submitEditClone">Lưu</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="previewPlayerVisible" title="Nghe thử âm thanh" width="560px" @close="closePreviewPlayerDialog">
      <div class="preview-player">
        <div class="preview-player-meta">
          <el-tag size="small" effect="plain">{{ previewPlayerSourceLabel || '-' }}</el-tag>
          <span class="preview-player-name">{{ previewPlayerCloneLabel || '-' }}</span>
        </div>
        <audio
          ref="previewPlayerRef"
          class="preview-player-audio"
          :src="previewPlayerURL"
          controls
          preload="metadata"
          @play="onPreviewAudioPlay"
          @pause="onPreviewAudioPause"
          @ended="onPreviewAudioEnded"
          @timeupdate="onPreviewAudioTimeUpdate"
          @loadedmetadata="onPreviewAudioLoadedMetadata"
        />
        <div class="preview-player-actions">
          <el-button type="primary" :disabled="!previewPlayerURL" @click="togglePreviewPlayback">
            {{ previewPlayerPlaying ? 'Tạm dừng' : 'Phát' }}
          </el-button>
          <el-button :disabled="!previewPlayerURL" @click="stopPreviewPlayback">Dừng</el-button>
          <span class="preview-player-time"
            >{{ formatPlayerTime(previewPlayerCurrentTime) }} / {{ formatPlayerTime(previewPlayerDuration) }}</span
          >
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, onBeforeUnmount, onMounted } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import api from '../../utils/api';
import { useAuthStore } from '../../stores/auth';

const authStore = useAuthStore();
const loading = ref(false);
const submitting = ref(false);
const createDialogVisible = ref(false);
const audioDialogVisible = ref(false);
const editDialogVisible = ref(false);
const voiceClones = ref([]);
const currentAudios = ref([]);
const ttsConfigs = ref([]);
const MIN_AUDIO_DURATION_SECONDS = 10;
const cloneEnabledProviders = ['doubao_ws', 'minimax', 'cosyvoice', 'aliyun_qwen', 'indextts_vllm'];
const pendingStatuses = ['queued', 'processing'];
let clonePollingTimer = null;
const clonePollingBusy = ref(false);
const editSubmitting = ref(false);
const retrySubmittingID = ref(null);
const previewUploadSubmittingID = ref(null);
const previewClonedSubmittingID = ref(null);
const appendAudioSubmittingID = ref(null);
const shareSubmittingID = ref(null);
const deleteSubmittingID = ref(null);
const appendAudioInputRef = ref(null);
const appendAudioTargetClone = ref(null);
const previewPlayerVisible = ref(false);
const previewPlayerRef = ref(null);
const previewPlayerURL = ref('');
const previewPlayerSourceLabel = ref('');
const previewPlayerCloneLabel = ref('');
const previewPlayerPlaying = ref(false);
const previewPlayerCurrentTime = ref(0);
const previewPlayerDuration = ref(0);

const form = ref({
  name: '',
  tts_config_id: '',
  source_type: 'upload',
  transcript: '',
  transcript_lang: 'vi-VN',
  audioFile: null,
  recordBlob: null,
  audioDurationSec: 0,
});

const editForm = ref({
  id: null,
  originalName: '',
  name: '',
  provider: '',
  ttsConfigDisplay: '',
  providerVoiceID: '',
  statusText: '',
  createdAtText: '',
  lastError: '',
});

const capability = ref({ enabled: true, requires_transcript: false, min_text_len: 0, max_text_len: 0 });

const cloneEnabledConfigs = computed(() =>
  ttsConfigs.value.filter((item) => cloneEnabledProviders.includes(item.provider)),
);
const selectedCloneConfig = computed(
  () => cloneEnabledConfigs.value.find((item) => item.config_id === form.value.tts_config_id) || null,
);
const currentCloneProvider = computed(() => selectedCloneConfig.value?.provider || '');
const normalizeProvider = (provider) =>
  String(provider || '')
    .trim()
    .toLowerCase();
const resolveChargeNotice = (provider, scene = 'create') => {
  const normalized = normalizeProvider(provider);
  if (normalized === 'aliyun_qwen') {
    return {
      message:
        scene === 'create'
          ? 'Lưu ý phí: Sao chép giọng Qwen tính phí theo giọng, 1 xu/giọng.'
          : 'Lưu ý phí: Sao chép giọng Qwen tính phí theo giọng, 1 xu/giọng, xác nhận để tiếp tục nghe thử.',
      type: 'warning',
    };
  }
  if (normalized === 'minimax') {
    return {
      message:
        scene === 'create'
          ? 'Lưu ý phí: Sao chép Minimax miễn phí, lần đầu nghe thử tính phí 9.9 tệ.'
          : 'Lưu ý phí: Sao chép Minimax miễn phí, nhưng lần đầu nghe thử tính phí 9.9 tệ, xác nhận để tiếp tục.',
      type: 'warning',
    };
  }
  if (normalized === 'cosyvoice') {
    return {
      message:
        scene === 'create'
          ? 'Lưu ý phí: Sao chép và nghe thử CosyVoice hoàn toàn miễn phí.'
          : 'Lưu ý phí: Sao chép và nghe thử CosyVoice miễn phí, xác nhận để tiếp tục.',
      type: 'info',
    };
  }
  return { message: '', type: 'info' };
};
const createChargeNotice = computed(() => resolveChargeNotice(currentCloneProvider.value, 'create'));
const requiresMinimaxDuration = computed(() => currentCloneProvider.value === 'minimax');
const isAliyunQwenProvider = computed(() => currentCloneProvider.value === 'aliyun_qwen');
const qwenCloneRuntimeModel = 'qwen3-tts-vc-2026-01-22';
const uploadAcceptTypes = computed(() => {
  if (isAliyunQwenProvider.value) {
    return '.wav,.mp3,.m4a,audio/wav,audio/wave,audio/mpeg,audio/mp4,audio/x-m4a';
  }
  return '.wav,audio/wav,audio/wave';
});
const audioRequirementText = computed(() => {
  if (requiresMinimaxDuration.value) {
    return `Yêu cầu: Định dạng WAV, thời lượng ít nhất ${MIN_AUDIO_DURATION_SECONDS} giây`;
  }
  if (isAliyunQwenProvider.value) {
    return 'Yêu cầu: WAV/MP3/M4A, khuyến nghị 10-20 giây (tối đa 60 giây)';
  }
  return 'Yêu cầu: Định dạng WAV (CosyVoice cần nhập văn bản tương ứng)';
});

const isRecording = ref(false);
const mediaRecorder = ref(null);
const recordChunks = ref([]);
const recordPreviewUrl = ref('');

const formatDate = (value) => (value ? new Date(value).toLocaleString() : '-');
const parseMetaJSON = (metaJSON) => {
  if (!metaJSON || typeof metaJSON !== 'string') return {};
  try {
    return JSON.parse(metaJSON);
  } catch (error) {
    return {};
  }
};
const normalizeCloneStatus = (row) => {
  const status = String(row?.status || '')
    .trim()
    .toLowerCase();
  const taskStatus = String(row?.task_status || '')
    .trim()
    .toLowerCase();
  if (status === 'failed' || taskStatus === 'failed') return 'failed';
  if (status === 'active' || taskStatus === 'succeeded') return 'active';
  if (taskStatus === 'queued' || taskStatus === 'processing') return taskStatus;
  if (status === 'queued' || status === 'processing') return status;
  return status || taskStatus || 'unknown';
};
const formatCloneStatus = (row) => {
  const status = normalizeCloneStatus(row);
  if (status === 'queued') return 'Đang xếp hàng';
  if (status === 'processing') return 'Đang xử lý';
  if (status === 'active') return 'Thành công';
  if (status === 'failed') return 'Thất bại';
  return 'Không rõ';
};
const getCloneStatusTagType = (row) => {
  const status = normalizeCloneStatus(row);
  if (status === 'queued') return 'info';
  if (status === 'processing') return 'warning';
  if (status === 'active') return 'success';
  if (status === 'failed') return 'danger';
  return 'info';
};
const getCloneLastError = (row) => {
  const status = normalizeCloneStatus(row);
  if (status !== 'failed') return '-';
  if (row?.task_last_error) return row.task_last_error;
  const meta = parseMetaJSON(row?.meta_json);
  return meta.last_error || '-';
};
const canRetryClone = (row) => normalizeCloneStatus(row) === 'failed';
const canPreviewClonedVoice = (row) => normalizeCloneStatus(row) === 'active';
const canAppendRefAudio = (row) =>
  normalizeCloneStatus(row) === 'active' && normalizeProvider(row?.provider) === 'indextts_vllm';
const formatPlayerTime = (seconds) => {
  const value = Number(seconds || 0);
  if (!Number.isFinite(value) || value < 0) return '00:00';
  const total = Math.floor(value);
  const minute = String(Math.floor(total / 60)).padStart(2, '0');
  const second = String(total % 60).padStart(2, '0');
  return `${minute}:${second}`;
};
const pauseAllOtherAudios = () => {
  const current = previewPlayerRef.value;
  document.querySelectorAll('audio').forEach((audioEl) => {
    if (audioEl !== current) {
      try {
        audioEl.pause();
      } catch (error) {
        // ignore pause errors from detached nodes
      }
    }
  });
};
const revokePreviewPlayerURL = () => {
  if (!previewPlayerURL.value) return;
  URL.revokeObjectURL(previewPlayerURL.value);
  previewPlayerURL.value = '';
};
const stopPreviewPlayback = () => {
  const audioEl = previewPlayerRef.value;
  if (!audioEl) return;
  audioEl.pause();
  audioEl.currentTime = 0;
  previewPlayerCurrentTime.value = 0;
};
const closePreviewPlayerDialog = () => {
  stopPreviewPlayback();
  previewPlayerPlaying.value = false;
  previewPlayerCurrentTime.value = 0;
  previewPlayerDuration.value = 0;
  previewPlayerSourceLabel.value = '';
  previewPlayerCloneLabel.value = '';
  revokePreviewPlayerURL();
};
const setPreviewPlayerSource = async (blob, sourceLabel, cloneLabel) => {
  stopPreviewPlayback();
  revokePreviewPlayerURL();
  previewPlayerURL.value = URL.createObjectURL(blob);
  previewPlayerSourceLabel.value = sourceLabel;
  previewPlayerCloneLabel.value = cloneLabel;
  previewPlayerCurrentTime.value = 0;
  previewPlayerDuration.value = 0;
  previewPlayerVisible.value = true;
  await nextTick();
  pauseAllOtherAudios();
  const audioEl = previewPlayerRef.value;
  if (!audioEl) return;
  try {
    await audioEl.play();
  } catch (error) {
    ElMessage.info('Âm thanh đã tải, nhấn phát để nghe thử');
  }
};
const togglePreviewPlayback = async () => {
  const audioEl = previewPlayerRef.value;
  if (!audioEl) return;
  if (audioEl.paused) {
    pauseAllOtherAudios();
    await audioEl.play();
    return;
  }
  audioEl.pause();
};
const onPreviewAudioPlay = () => {
  pauseAllOtherAudios();
  previewPlayerPlaying.value = true;
};
const onPreviewAudioPause = () => {
  previewPlayerPlaying.value = false;
};
const onPreviewAudioEnded = () => {
  previewPlayerPlaying.value = false;
  previewPlayerCurrentTime.value = previewPlayerDuration.value;
};
const onPreviewAudioTimeUpdate = () => {
  const audioEl = previewPlayerRef.value;
  if (!audioEl) return;
  previewPlayerCurrentTime.value = Number(audioEl.currentTime || 0);
};
const onPreviewAudioLoadedMetadata = () => {
  const audioEl = previewPlayerRef.value;
  if (!audioEl) return;
  previewPlayerDuration.value = Number(audioEl.duration || 0);
};
const hasPendingCloneTask = (row) => pendingStatuses.includes(normalizeCloneStatus(row));
const clearClonePollingTimer = () => {
  if (!clonePollingTimer) return;
  window.clearTimeout(clonePollingTimer);
  clonePollingTimer = null;
};
const scheduleClonePolling = () => {
  if (clonePollingTimer) return;
  clonePollingTimer = window.setTimeout(async () => {
    clonePollingTimer = null;
    if (!voiceClones.value.some(hasPendingCloneTask)) return;
    if (clonePollingBusy.value) {
      scheduleClonePolling();
      return;
    }
    clonePollingBusy.value = true;
    try {
      await loadVoiceClones(true);
    } finally {
      clonePollingBusy.value = false;
      if (voiceClones.value.some(hasPendingCloneTask)) {
        scheduleClonePolling();
      }
    }
  }, 2000);
};

const loadVoiceClones = async (silent = false) => {
  if (!silent) loading.value = true;
  try {
    const res = await api.get('/user/voice-clones');
    voiceClones.value = res.data.data || [];
  } finally {
    if (!silent) loading.value = false;
    if (voiceClones.value.some(hasPendingCloneTask)) {
      scheduleClonePolling();
    } else {
      clearClonePollingTimer();
    }
  }
};

const loadTtsConfigs = async () => {
  const res = await api.get('/user/tts-configs');
  ttsConfigs.value = res.data.data || [];
};

const openCreateDialog = async () => {
  createDialogVisible.value = true;
  await loadTtsConfigs();
  if (!cloneEnabledConfigs.value.length) {
    form.value.tts_config_id = '';
    return;
  }
  const selectedConfig = cloneEnabledConfigs.value.find((item) => item.config_id === form.value.tts_config_id);
  if (!selectedConfig) {
    form.value.tts_config_id = cloneEnabledConfigs.value[0].config_id;
  }
  await onConfigChange(form.value.tts_config_id);
};

const onConfigChange = async (configId) => {
  const cfg = cloneEnabledConfigs.value.find((item) => item.config_id === configId);
  if (!cfg) {
    capability.value = { enabled: true, requires_transcript: false, min_text_len: 0, max_text_len: 0 };
    return;
  }
  const res = await api.get('/user/voice-clone/capabilities', { params: { provider: cfg.provider } });
  capability.value = res.data.data || capability.value;
};

const isWavFile = (file) => {
  const name = (file?.name || '').toLowerCase();
  const type = (file?.type || '').toLowerCase();
  return type.includes('audio/wav') || type.includes('audio/wave') || name.endsWith('.wav');
};

const isSupportedAliyunQwenAudio = (file) => {
  const name = (file?.name || '').toLowerCase();
  const type = (file?.type || '').toLowerCase();
  if (name.endsWith('.wav') || name.endsWith('.mp3') || name.endsWith('.m4a')) {
    return true;
  }
  return (
    type.includes('audio/wav') ||
    type.includes('audio/wave') ||
    type.includes('audio/mpeg') ||
    type.includes('audio/mp4') ||
    type.includes('audio/x-m4a')
  );
};

const isSupportedUploadAudio = (file) => {
  if (isAliyunQwenProvider.value) {
    return isSupportedAliyunQwenAudio(file);
  }
  return isWavFile(file);
};

const getAudioDurationSeconds = (blobOrFile) =>
  new Promise((resolve, reject) => {
    const url = URL.createObjectURL(blobOrFile);
    const audio = new Audio();
    audio.preload = 'metadata';
    audio.onloadedmetadata = () => {
      const duration = Number(audio.duration || 0);
      URL.revokeObjectURL(url);
      if (!Number.isFinite(duration) || duration <= 0) {
        reject(new Error('Không thể đọc thời lượng âm thanh'));
        return;
      }
      resolve(duration);
    };
    audio.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('Không thể phân tích tệp âm thanh'));
    };
    audio.src = url;
  });

const handleFileChange = async (event) => {
  const file = event.target.files?.[0] || null;
  if (!file) {
    form.value.audioFile = null;
    form.value.audioDurationSec = 0;
    return;
  }
  if (!isSupportedUploadAudio(file)) {
    ElMessage.warning(isAliyunQwenProvider.value ? 'Chỉ hỗ trợ âm thanh WAV/MP3/M4A' : 'Chỉ hỗ trợ định dạng WAV');
    form.value.audioFile = null;
    form.value.audioDurationSec = 0;
    event.target.value = '';
    return;
  }
  if (!requiresMinimaxDuration.value) {
    form.value.audioFile = file;
    form.value.audioDurationSec = 0;
    return;
  }
  try {
    const duration = await getAudioDurationSeconds(file);
    if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
      ElMessage.warning(
        `Thời lượng âm thanh phải ít nhất ${MIN_AUDIO_DURATION_SECONDS} giây, hiện tại khoảng ${duration.toFixed(2)} giây`,
      );
      form.value.audioFile = null;
      form.value.audioDurationSec = 0;
      event.target.value = '';
      return;
    }
    form.value.audioFile = file;
    form.value.audioDurationSec = duration;
  } catch (error) {
    ElMessage.warning(error.message || 'Đọc thời lượng âm thanh thất bại');
    form.value.audioFile = null;
    form.value.audioDurationSec = 0;
    event.target.value = '';
  }
};

const convertToWav = async (blob) => {
  const arrayBuffer = await blob.arrayBuffer();
  const audioContext = new (window.AudioContext || window.webkitAudioContext)();
  try {
    const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);
    const wav = audioBufferToWav(audioBuffer);
    return new Blob([wav], { type: 'audio/wav' });
  } finally {
    await audioContext.close();
  }
};

const audioBufferToWav = (buffer) => {
  const length = buffer.length;
  const numberOfChannels = buffer.numberOfChannels;
  const sampleRate = buffer.sampleRate;
  const bytesPerSample = 2;
  const blockAlign = numberOfChannels * bytesPerSample;
  const byteRate = sampleRate * blockAlign;
  const dataSize = length * blockAlign;
  const bufferSize = 44 + dataSize;
  const arrayBuffer = new ArrayBuffer(bufferSize);
  const view = new DataView(arrayBuffer);
  const writeString = (offset, str) => {
    for (let i = 0; i < str.length; i += 1) {
      view.setUint8(offset + i, str.charCodeAt(i));
    }
  };

  writeString(0, 'RIFF');
  view.setUint32(4, bufferSize - 8, true);
  writeString(8, 'WAVE');
  writeString(12, 'fmt ');
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, numberOfChannels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, 16, true);
  writeString(36, 'data');
  view.setUint32(40, dataSize, true);

  let offset = 44;
  for (let i = 0; i < length; i += 1) {
    for (let channel = 0; channel < numberOfChannels; channel += 1) {
      const sample = Math.max(-1, Math.min(1, buffer.getChannelData(channel)[i]));
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
      offset += 2;
    }
  }
  return arrayBuffer;
};

const startRecording = async () => {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  recordChunks.value = [];
  form.value.audioDurationSec = 0;
  const recorderOptions = { mimeType: 'audio/webm;codecs=opus' };
  const recorder = MediaRecorder.isTypeSupported(recorderOptions.mimeType)
    ? new MediaRecorder(stream, recorderOptions)
    : new MediaRecorder(stream);
  mediaRecorder.value = recorder;
  recorder.ondataavailable = (evt) => {
    if (evt.data && evt.data.size > 0) recordChunks.value.push(evt.data);
  };
  recorder.onstop = async () => {
    const blob = new Blob(recordChunks.value, { type: recordChunks.value[0]?.type || 'audio/webm' });
    try {
      const wavBlob = await convertToWav(blob);
      const duration = await getAudioDurationSeconds(wavBlob);
      if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
        ElMessage.warning(
          `Thời lượng ghi âm phải ít nhất ${MIN_AUDIO_DURATION_SECONDS} giây, hiện tại khoảng ${duration.toFixed(2)} giây`,
        );
        form.value.recordBlob = null;
        form.value.audioDurationSec = 0;
        if (recordPreviewUrl.value) {
          URL.revokeObjectURL(recordPreviewUrl.value);
          recordPreviewUrl.value = '';
        }
      } else {
        form.value.recordBlob = wavBlob;
        form.value.audioDurationSec = duration;
        if (recordPreviewUrl.value) URL.revokeObjectURL(recordPreviewUrl.value);
        recordPreviewUrl.value = URL.createObjectURL(wavBlob);
      }
    } catch (error) {
      ElMessage.error('Chuyển đổi ghi âm thất bại, vui lòng thử lại');
      form.value.recordBlob = null;
      form.value.audioDurationSec = 0;
      if (recordPreviewUrl.value) {
        URL.revokeObjectURL(recordPreviewUrl.value);
        recordPreviewUrl.value = '';
      }
    }
    stream.getTracks().forEach((t) => t.stop());
  };
  recorder.start();
  isRecording.value = true;
};

const stopRecording = () => {
  if (mediaRecorder.value) mediaRecorder.value.stop();
  isRecording.value = false;
};

const submitClone = async () => {
  if (!form.value.tts_config_id) {
    ElMessage.warning('Vui lòng chọn cấu hình TTS có thể sao chép');
    return;
  }
  const createNotice = resolveChargeNotice(currentCloneProvider.value, 'create');
  if (createNotice.message) {
    try {
      await ElMessageBox.confirm(createNotice.message, 'Nhắc nhở tạo sao chép', {
        confirmButtonText: 'Tôi đã hiểu, tiếp tục',
        cancelButtonText: 'Hủy',
        type: createNotice.type,
      });
    } catch (error) {
      return;
    }
  }
  if (capability.value.requires_transcript && !form.value.transcript.trim()) {
    ElMessage.warning('Nhà cung cấp này yêu cầu nhập văn bản âm thanh');
    return;
  }

  const fd = new FormData();
  fd.append('name', form.value.name);
  fd.append('tts_config_id', form.value.tts_config_id);
  fd.append('source_type', form.value.source_type);
  fd.append('transcript', form.value.transcript);
  fd.append('transcript_lang', form.value.transcript_lang);

  if (form.value.source_type === 'upload') {
    if (!form.value.audioFile) {
      ElMessage.warning('Vui lòng tải lên tệp âm thanh');
      return;
    }
    let duration = form.value.audioDurationSec;
    if (requiresMinimaxDuration.value && !duration) {
      try {
        duration = await getAudioDurationSeconds(form.value.audioFile);
      } catch (error) {
        ElMessage.warning(error.message || 'Đọc thời lượng âm thanh thất bại');
        return;
      }
    }
    if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
      ElMessage.warning(
        `Thời lượng âm thanh phải ít nhất ${MIN_AUDIO_DURATION_SECONDS} giây, hiện tại khoảng ${duration.toFixed(2)} giây`,
      );
      return;
    }
    fd.append('audio_file', form.value.audioFile);
  } else {
    if (!form.value.recordBlob) {
      ElMessage.warning('Vui lòng ghi âm trước');
      return;
    }
    let duration = form.value.audioDurationSec;
    if (requiresMinimaxDuration.value && !duration) {
      try {
        duration = await getAudioDurationSeconds(form.value.recordBlob);
      } catch (error) {
        ElMessage.warning(error.message || 'Đọc thời lượng ghi âm thất bại');
        return;
      }
    }
    if (requiresMinimaxDuration.value && duration < MIN_AUDIO_DURATION_SECONDS) {
      ElMessage.warning(
        `Thời lượng ghi âm phải ít nhất ${MIN_AUDIO_DURATION_SECONDS} giây, hiện tại khoảng ${duration.toFixed(2)} giây`,
      );
      return;
    }
    fd.append('audio_blob', form.value.recordBlob, `recording_${Date.now()}.wav`);
  }

  submitting.value = true;
  try {
    const res = await api.post('/user/voice-clones', fd, { timeout: 120000 });
    const queued = res.status === 202 || pendingStatuses.includes(normalizeCloneStatus(res.data?.data || {}));
    ElMessage.success(queued ? 'Đã gửi tác vụ sao chép, đang xử lý nền' : 'Tạo giọng sao chép thành công');
    createDialogVisible.value = false;
    await loadVoiceClones();
  } finally {
    submitting.value = false;
  }
};

const loadAudios = async (clone) => {
  const res = await api.get(`/user/voice-clones/${clone.id}/audios`);
  currentAudios.value = res.data.data || [];
  audioDialogVisible.value = true;
};

const openEditDialog = (clone) => {
  if (!clone) return;
  editForm.value = {
    id: clone.id,
    originalName: String(clone.name || ''),
    name: String(clone.name || ''),
    provider: String(clone.provider || '-'),
    ttsConfigDisplay: `${clone.tts_config_name || '-'} (${clone.tts_config_id || '-'})`,
    providerVoiceID: String(clone.provider_voice_id || '-'),
    statusText: formatCloneStatus(clone),
    createdAtText: formatDate(clone.created_at),
    lastError: String(getCloneLastError(clone) === '-' ? '' : getCloneLastError(clone)),
  };
  editDialogVisible.value = true;
};

const resetEditForm = () => {
  editForm.value = {
    id: null,
    originalName: '',
    name: '',
    provider: '',
    ttsConfigDisplay: '',
    providerVoiceID: '',
    statusText: '',
    createdAtText: '',
    lastError: '',
  };
  editSubmitting.value = false;
};

const submitEditClone = async () => {
  const cloneID = editForm.value.id;
  if (!cloneID) return;
  const nextName = String(editForm.value.name || '').trim();
  if (!nextName) {
    ElMessage.warning('Tên không được để trống');
    return;
  }
  if ([...nextName].length > 100) {
    ElMessage.warning('Tên không được vượt quá 100 ký tự');
    return;
  }
  if (nextName === String(editForm.value.originalName || '').trim()) {
    editDialogVisible.value = false;
    return;
  }

  editSubmitting.value = true;
  try {
    await api.put(`/user/voice-clones/${cloneID}`, { name: nextName });
    ElMessage.success('Cập nhật tên thành công');
    editDialogVisible.value = false;
    await loadVoiceClones(true);
  } finally {
    editSubmitting.value = false;
  }
};

const retryClone = async (clone) => {
  if (!clone?.id || !canRetryClone(clone) || retrySubmittingID.value) return;
  retrySubmittingID.value = clone.id;
  try {
    await api.post(`/user/voice-clones/${clone.id}/retry`);
    ElMessage.success('Đã gửi tác vụ sao chép lại, đang xử lý nền');
    await loadVoiceClones(true);
  } finally {
    retrySubmittingID.value = null;
  }
};

const toggleSharedToAll = async (clone, nextValue) => {
  if (!authStore.isAdmin || !clone?.id) return;
  shareSubmittingID.value = clone.id;
  try {
    await api.put(`/user/voice-clones/${clone.id}`, { shared_to_all: !!nextValue });
    clone.shared_to_all = !!nextValue;
    ElMessage.success(nextValue ? 'Đã bật chia sẻ cho tất cả' : 'Đã tắt chia sẻ');
  } finally {
    shareSubmittingID.value = null;
  }
};

const deleteClone = async (clone) => {
  if (!clone?.id || deleteSubmittingID.value) return;
  try {
    await ElMessageBox.confirm(
      `Xác nhận xóa giọng sao chép“${clone.name || clone.provider_voice_id || clone.id}” không? Sau khi xóa sẽ không còn xuất hiện trong danh sách.`,
      'Xóa giọng đọc sao chép',
      {
        type: 'warning',
        confirmButtonText: 'Xóa',
        cancelButtonText: 'Hủy',
      },
    );
  } catch {
    return;
  }
  deleteSubmittingID.value = clone.id;
  try {
    await api.delete(`/user/voice-clones/${clone.id}`);
    ElMessage.success('Xóa thành công');
    await loadVoiceClones(true);
  } finally {
    deleteSubmittingID.value = null;
  }
};

const openAppendAudioDialog = (clone) => {
  if (!clone?.id || !canAppendRefAudio(clone) || appendAudioSubmittingID.value) return;
  appendAudioTargetClone.value = clone;
  const input = appendAudioInputRef.value;
  if (!input) {
    ElMessage.error('Bộ chọn tệp chưa sẵn sàng');
    return;
  }
  input.value = '';
  input.click();
};

const handleAppendAudioFileChange = async (event) => {
  const file = event?.target?.files?.[0];
  const clone = appendAudioTargetClone.value;
  if (!file || !clone?.id) {
    appendAudioTargetClone.value = null;
    return;
  }
  appendAudioSubmittingID.value = clone.id;
  try {
    const fd = new FormData();
    fd.append('source_type', 'upload');
    fd.append('audio_file', file);
    await api.post(`/user/voice-clones/${clone.id}/append-audio`, fd, { timeout: 120000 });
    ElMessage.success('Thêm âm thanh tham chiếu thành công');
    await loadVoiceClones(true);
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || 'Thêm âm thanh tham chiếu thất bại');
  } finally {
    appendAudioSubmittingID.value = null;
    appendAudioTargetClone.value = null;
    if (event?.target) event.target.value = '';
  }
};

const playAudio = async (audio) => {
  const response = await api.get(`/user/voice-clones/audios/${audio.id}/file`, { responseType: 'blob' });
  const label = String(audio?.file_name || '');
  await setPreviewPlayerSource(response.data, 'Âm thanh gốc', label || 'Âm thanh gốc sao chép');
};

const previewUploadedAudio = async (clone) => {
  if (!clone?.id || previewUploadSubmittingID.value) return;
  previewUploadSubmittingID.value = clone.id;
  try {
    const res = await api.get(`/user/voice-clones/${clone.id}/audios`);
    const audios = res.data.data || [];
    if (!audios.length) {
      ElMessage.warning('Không tìm thấy âm thanh đã tải lên');
      return;
    }
    const audioRes = await api.get(`/user/voice-clones/audios/${audios[0].id}/file`, { responseType: 'blob' });
    await setPreviewPlayerSource(audioRes.data, 'Âm thanh gốc', String(clone?.name || 'Tác vụ sao chép'));
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || 'Xem trước âm thanh tải lên thất bại');
  } finally {
    previewUploadSubmittingID.value = null;
  }
};

const previewClonedVoice = async (clone) => {
  if (!clone?.id || !canPreviewClonedVoice(clone) || previewClonedSubmittingID.value) return;
  const previewNotice = resolveChargeNotice(clone?.provider, 'preview');
  if (previewNotice.message) {
    try {
      await ElMessageBox.confirm(previewNotice.message, 'Nhắc nhở nghe thử sao chép', {
        confirmButtonText: 'Tiếp tục nghe thử',
        cancelButtonText: 'Hủy',
        type: previewNotice.type,
      });
    } catch (error) {
      return;
    }
  }
  previewClonedSubmittingID.value = clone.id;
  try {
    const response = await api.get(`/user/voice-clones/${clone.id}/preview`, { responseType: 'blob' });
    await setPreviewPlayerSource(response.data, 'Nghe thử sao chép', String(clone?.name || 'Tác vụ sao chép'));
  } catch (error) {
    ElMessage.error(error?.response?.data?.error || 'Nghe thử âm thanh sao chép thất bại');
  } finally {
    previewClonedSubmittingID.value = null;
  }
};

onMounted(async () => {
  await loadVoiceClones();
});

onBeforeUnmount(() => {
  clearClonePollingTimer();
  closePreviewPlayerDialog();
});
</script>

<style scoped>
.voice-clones-page {
  padding: 20px;
}
.page-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.help {
  color: #999;
  font-size: 12px;
  margin-top: 4px;
}

.clone-charge-alert {
  margin-top: 8px;
}

.voice-clones-page :deep(.el-table .cell) {
  white-space: nowrap;
}

.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.action-buttons :deep(.el-button) {
  margin: 0;
  white-space: nowrap;
}

.readonly-field :deep(.el-input__wrapper) {
  background-color: var(--el-fill-color-light);
  box-shadow: 0 0 0 1px var(--el-border-color-light) inset;
}

.readonly-field :deep(.el-input__inner) {
  color: var(--el-text-color-secondary);
}

.readonly-field :deep(.el-textarea__inner) {
  background-color: var(--el-fill-color-light);
  border-color: var(--el-border-color-light);
  color: var(--el-text-color-secondary);
}

.preview-player {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.preview-player-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.preview-player-name {
  color: var(--el-text-color-regular);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-player-audio {
  width: 100%;
}

.preview-player-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.preview-player-time {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
