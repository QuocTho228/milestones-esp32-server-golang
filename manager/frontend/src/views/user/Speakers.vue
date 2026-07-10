<template>
  <div class="speakers-page">
    <!-- Thanh lọc -->
    <div class="filter-bar">
      <el-select
        v-model="filterAgentId"
        placeholder="Lọc theo trợ lý AI"
        clearable
        style="width: 200px; margin-right: 10px"
        @change="loadSpeakerGroups"
      >
        <el-option label="Tất cả trợ lý AI" value="" />
        <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
      </el-select>
      <el-input
        v-model="searchKeyword"
        placeholder="Tìm kiếm tên nhóm giọng nói"
        clearable
        style="width: 250px"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button class="create-group-button" type="primary" @click="handleAddGroup">
        <el-icon><Plus /></el-icon>
        Tạo nhóm giọng nói
      </el-button>
    </div>

    <!-- Danh sách nhóm giọng nói -->
    <div v-loading="loading" class="speakers-content">
      <el-table :data="filteredGroups" stripe style="width: 100%">
        <el-table-column prop="name" label="Tên nhóm giọng nói" min-width="150" />
        <el-table-column prop="agent_name" label="Trợ lý AI liên kết" min-width="120" />
        <el-table-column label="Prompt" min-width="200">
          <template #default="{ row }">
            <el-popover placement="top" :width="300" trigger="hover" v-if="row.prompt">
              <template #reference>
                <span class="prompt-text">{{ truncateText(row.prompt, 30) }}</span>
              </template>
              <div class="prompt-popover">{{ row.prompt }}</div>
            </el-popover>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="sample_count" label="Số mẫu" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info">{{ row.sample_count }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="Thời gian tạo" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="Thao tác" width="360" fixed="right">
          <template #default="{ row }">
            <div class="action-buttons">
              <el-button type="success" size="small" @click="handleVerifyGroup(row)">
                <el-icon><VideoPlay /></el-icon>
                Xác minh
              </el-button>
              <el-button type="primary" size="small" @click="handleViewSamples(row)">
                <el-icon><View /></el-icon>
                Quản lý giọng nói
              </el-button>
              <el-button type="primary" size="small" plain @click="handleEditGroup(row)">
                <el-icon><Edit /></el-icon>
                Chỉnh sửa
              </el-button>
              <el-button type="danger" size="small" @click="handleDeleteGroup(row)">
                <el-icon><Delete /></el-icon>
                Xóa
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="filteredGroups.length === 0 && !loading" class="empty-state">
        <el-empty description="Chưa có dữ liệu nhóm giọng nói" />
      </div>
    </div>

    <!-- Hộp thoại tạo/chỉnh sửa nhóm giọng nói -->
    <el-dialog
      v-model="showGroupDialog"
      :title="groupDialogMode === 'add' ? 'Tạo nhóm giọng nói' : 'Chỉnh sửa nhóm giọng nói'"
      width="720px"
    >
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="150px">
        <el-form-item label="Trợ lý AI liên kết" prop="agent_id">
          <el-select v-model="groupForm.agent_id" placeholder="Chọn trợ lý AI" style="width: 100%">
            <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="Tên giọng nói" prop="name">
          <el-input v-model="groupForm.name" placeholder="Nhập tên giọng nói" :maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="Prompt" prop="prompt">
          <el-input
            v-model="groupForm.prompt"
            type="textarea"
            :rows="4"
            placeholder="Nhập từ gợi ý vai trò (tuỳ chọn)"
          />
        </el-form-item>
        <el-form-item label="Mô tả" prop="description">
          <el-input
            v-model="groupForm.description"
            type="textarea"
            :rows="3"
            placeholder="Nhập mô tả (tuỳ chọn)"
            :maxlength="200"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="Giọng đọc tôi đã sao chép" v-if="cloneVoicePresets.length > 0">
          <div class="clone-voice-line" v-loading="cloneVoicesLoading">
            <button
              v-for="clone in cloneVoicePresets"
              :key="clone.id"
              type="button"
              class="clone-voice-item"
              :class="{ active: isCloneVoiceSelected(clone) }"
              :title="`${clone.tts_config_name || clone.tts_config_id} · ${clone.provider_voice_id}`"
              @click="applyCloneVoice(clone)"
            >
              <span class="clone-voice-name">{{ clone.name || clone.provider_voice_id }}</span>
            </button>
          </div>
          <div class="form-help">Nhấn để tự động điền cấu hình TTS và giọng đọc</div>
        </el-form-item>
        <el-form-item label="Cấu hình TTS" prop="tts_config_id">
          <el-select
            v-model="groupForm.tts_config_id"
            placeholder="Chọn cấu hình TTS (tuỳ chọn)"
            clearable
            style="width: 100%"
            @change="handleTtsConfigChange"
          >
            <el-option
              v-for="ttsConfig in ttsConfigs"
              :key="ttsConfig.config_id"
              :label="ttsConfig.is_default ? `${ttsConfig.name} (Mặc định)` : ttsConfig.name"
              :value="ttsConfig.config_id"
            >
              <div class="config-option">
                {{ ttsConfig.name }}
                <el-tag v-if="ttsConfig.is_default" type="success" size="small" style="margin-left: 8px"
                  >Mặc định</el-tag
                >
              </div>
              <span class="config-desc">{{ ttsConfig.provider || 'Chưa có mô tả' }}</span>
            </el-option>
          </el-select>
          <div class="form-help" v-if="groupForm.tts_config_id">
            {{ getCurrentTtsConfigInfo() }}
          </div>
        </el-form-item>
        <el-form-item label="Giọng đọc" prop="voice" v-if="groupForm.tts_config_id">
          <el-select
            v-model="groupForm.voice"
            placeholder="Chọn hoặc nhập giọng đọc"
            filterable
            allow-create
            clearable
            style="width: 100%"
          >
            <el-option
              v-for="voice in currentVoiceOptions"
              :key="voice.value"
              :label="voice.label"
              :value="voice.value"
            />
          </el-select>
          <div class="form-help">
            Cấu hình TTS hiện tại: {{ getCurrentTtsConfigName() }}, có thể tìm kiếm tên hoặc giá trị giọng đọc, hoặc
            nhập tùy chỉnh.
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showGroupDialog = false">Hủy</el-button>
        <el-button type="primary" @click="handleSubmitGroup" :loading="submitting">
          {{ groupDialogMode === 'add' ? 'Tạo' : 'Lưu' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Ngăn kéo quản lý mẫu -->
    <el-drawer v-model="showSampleDrawer" title="Quản lý mẫu" :size="800" :before-close="handleCloseSampleDrawer">
      <div v-if="currentGroup" class="sample-drawer">
        <!-- Thông tin nhóm giọng nói -->
        <el-card class="group-info-card" shadow="never">
          <div class="group-info">
            <h3>{{ currentGroup.name }}</h3>
            <div v-if="currentGroup.prompt" class="prompt-section">
              <strong>Prompt:</strong>
              <p>{{ currentGroup.prompt }}</p>
            </div>
            <div v-if="currentGroup.description" class="description-section">
              <strong>Mô tả:</strong>
              <p>{{ currentGroup.description }}</p>
            </div>
          </div>
        </el-card>

        <!-- Danh sách mẫu -->
        <div class="samples-section">
          <div class="samples-header">
            <h4>Danh sách mẫu</h4>
            <div class="samples-header-actions">
              <el-button type="success" @click="handleVerifyFromSamples">
                <el-icon><VideoPlay /></el-icon>
                Xác minh giọng nói
              </el-button>
              <el-button type="primary" @click="handleAddSample">
                <el-icon><Plus /></el-icon>
                Tải lên mẫu mới
              </el-button>
            </div>
          </div>

          <el-table :data="samples" stripe style="width: 100%">
            <el-table-column prop="uuid" label="UUID" min-width="200">
              <template #default="{ row }">
                <el-tooltip :content="row.uuid" placement="top">
                  <span class="uuid-text">{{ truncateId(row.uuid) }}</span>
                </el-tooltip>
                <el-button type="text" size="small" @click="copyToClipboard(row.uuid)" style="margin-left: 8px">
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </template>
            </el-table-column>
            <el-table-column prop="file_name" label="Tên tệp" min-width="150" />
            <el-table-column prop="file_size" label="Kích thước tệp" width="100">
              <template #default="{ row }">
                {{ formatFileSize(row.file_size) }}
              </template>
            </el-table-column>
            <el-table-column prop="duration" label="Thời lượng" width="80">
              <template #default="{ row }">
                {{ row.duration ? row.duration + 's' : '-' }}
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="Thời gian tạo" width="180">
              <template #default="{ row }">
                {{ formatDate(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column label="Thao tác" width="180" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" size="small" link @click="handlePlaySample(row)">
                  <el-icon><VideoPlay /></el-icon>
                  Phát
                </el-button>
                <el-button type="primary" size="small" link @click="handleDownloadSample(row)">
                  <el-icon><Download /></el-icon>
                  Tải xuống
                </el-button>
                <el-button type="danger" size="small" link @click="handleDeleteSample(row)">
                  <el-icon><Delete /></el-icon>
                  Xóa
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <div v-if="samples.length === 0" class="empty-samples">
            <el-empty description="Chưa có mẫu, vui lòng tải lên tệp âm thanh" />
          </div>
        </div>
      </div>
    </el-drawer>

    <!-- Hộp thoại tải lên mẫu -->
    <el-dialog
      v-model="showUploadDialog"
      title="Thêm mẫu giọng nói"
      width="600px"
      :before-close="handleCloseUploadDialog"
    >
      <el-tabs v-model="uploadMode" class="upload-tabs">
        <!-- Chọn từ lịch sử -->
        <el-tab-pane label="Chọn từ lịch sử" name="history">
          <div class="history-section">
            <el-form :model="historyForm" label-width="100px">
              <el-form-item label="Trợ lý AI">
                <el-select
                  v-model="historyForm.agent_id"
                  placeholder="Chọn trợ lý AI"
                  style="width: 100%"
                  @change="loadHistoryMessages"
                  clearable
                >
                  <el-option v-for="agent in agents" :key="agent.id" :label="agent.name" :value="agent.id" />
                </el-select>
              </el-form-item>
            </el-form>

            <div v-loading="loadingHistory" class="history-list">
              <div v-if="historyMessages.length === 0 && !loadingHistory" class="empty-history">
                <el-empty description="Chưa có lịch sử trò chuyện, vui lòng chọn trợ lý AI trước" />
              </div>
              <el-table
                v-else
                :data="historyMessages"
                row-key="message_id"
                stripe
                style="width: 100%"
                max-height="400"
                @row-click="handleSelectHistoryMessage"
              >
                <el-table-column label="Chọn" width="80" align="center">
                  <template #default="{ row }">
                    <el-radio
                      :model-value="historyForm.selected_message_id"
                      :label="row.message_id"
                      @change="historyForm.selected_message_id = row.message_id"
                    />
                  </template>
                </el-table-column>
                <el-table-column prop="content" label="Nội dung tin nhắn" min-width="200">
                  <template #default="{ row }">
                    <div class="message-content">{{ truncateText(row.content, 50) }}</div>
                  </template>
                </el-table-column>
                <el-table-column prop="device_id" label="ID thiết bị" width="150">
                  <template #default="{ row }">
                    <el-tooltip :content="row.device_id" placement="top">
                      <span>{{ truncateId(row.device_id) }}</span>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column prop="created_at" label="Thời gian" width="180">
                  <template #default="{ row }">
                    {{ formatDate(row.created_at) }}
                  </template>
                </el-table-column>
                <el-table-column label="Thao tác" width="100">
                  <template #default="{ row }">
                    <el-button type="primary" size="small" link @click.stop="handlePreviewHistoryAudio(row)">
                      <el-icon><VideoPlay /></el-icon>
                      Nghe thử
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </el-tab-pane>

        <!-- Tải lên tệp -->
        <el-tab-pane label="Tải lên tệp" name="upload">
          <el-form ref="uploadFormRef" :model="uploadForm" :rules="uploadRules" label-width="0">
            <el-form-item prop="audio">
              <el-upload
                ref="uploadRef"
                :auto-upload="false"
                :on-change="handleFileChange"
                :on-remove="handleFileRemove"
                :limit="1"
                accept=".wav,audio/wav"
                drag
                class="audio-upload"
              >
                <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                <div class="el-upload__text">Kéo tệp WAV vào đây, hoặc <em>nhấn để chọn tệp</em></div>
                <template #tip>
                  <div class="el-upload__tip">Chỉ hỗ trợ định dạng WAV, khuyến nghị 3-10 giây, không quá 10MB</div>
                </template>
              </el-upload>
              <div v-if="uploadForm.audioFile" class="file-info">
                <el-icon><Document /></el-icon>
                <span>{{ uploadForm.audioFile.name }}</span>
                <span class="file-size">({{ formatFileSize(uploadForm.audioFile.size) }})</span>
              </div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Ghi âm -->
        <el-tab-pane label="Ghi âm" name="record">
          <div class="record-section">
            <div class="record-status">
              <div v-if="!isRecording && !recordedBlob" class="record-ready">
                <el-icon size="48" color="var(--apple-primary)"><Microphone /></el-icon>
                <p>Nhấn nút bên dưới để bắt đầu ghi âm</p>
                <p class="record-tip">Khuyến nghị ghi âm 3-10 giây rõ ràng</p>
              </div>
              <div v-else-if="isRecording" class="record-recording">
                <div class="recording-indicator">
                  <span class="recording-dot"></span>
                  <span class="recording-text">Đang ghi âm...</span>
                </div>
                <div class="record-time">{{ formatRecordTime(recordTime) }}</div>
                <p class="record-tip">Nhấn nút dừng để kết thúc ghi âm</p>
              </div>
              <div v-else-if="recordedBlob" class="record-complete">
                <el-icon size="48" color="var(--apple-success)"><CircleCheck /></el-icon>
                <p>Ghi âm hoàn tất</p>
                <p class="record-tip">Thời lượng: {{ formatRecordTime(recordTime) }}</p>
                <audio :src="recordedBlobUrl" controls class="record-preview"></audio>
              </div>
            </div>

            <div class="record-controls">
              <el-button
                v-if="!isRecording && !recordedBlob"
                type="primary"
                size="large"
                @click="startRecording"
                :disabled="!canRecord"
              >
                <el-icon><VideoPlay /></el-icon>
                Bắt đầu ghi âm
              </el-button>
              <el-button v-if="isRecording" type="danger" size="large" @click="stopRecording">
                <el-icon><VideoPause /></el-icon>
                Dừng ghi âm
              </el-button>
              <el-button v-if="recordedBlob" type="primary" size="large" @click="startRecording" :disabled="!canRecord">
                <el-icon><Refresh /></el-icon>
                Ghi lại
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="handleCloseUploadDialog">Hủy</el-button>
        <el-button type="primary" @click="handleSubmitSample" :loading="submitting" :disabled="!hasAudioFile">
          Xác nhận
        </el-button>
      </template>
    </el-dialog>

    <!-- Hộp thoại xác minh nhóm giọng nói -->
    <el-dialog
      v-model="showVerifyDialog"
      :title="`Xác minh nhóm giọng nói: ${currentVerifyGroup?.name || ''}`"
      width="600px"
      :before-close="handleCloseVerifyDialog"
    >
      <el-tabs v-model="verifyMode" class="verify-tabs">
        <!-- Tải lên tệp -->
        <el-tab-pane label="Tải lên tệp" name="upload">
          <el-form ref="verifyFormRef" :model="verifyForm" :rules="verifyRules" label-width="0">
            <el-form-item prop="audio">
              <el-upload
                ref="verifyUploadRef"
                :auto-upload="false"
                :on-change="handleVerifyFileChange"
                :on-remove="handleVerifyFileRemove"
                :limit="1"
                accept=".wav,audio/wav"
                drag
                class="audio-upload"
                :file-list="verifyFileList"
              >
                <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                <div class="el-upload__text">Kéo tệp WAV vào đây, hoặc <em>nhấn để chọn tệp</em></div>
                <template #tip>
                  <div class="el-upload__tip">Chỉ hỗ trợ định dạng WAV, khuyến nghị 3-10 giây, không quá 10MB</div>
                </template>
              </el-upload>
              <div v-if="verifyForm.audioFile" class="file-info">
                <el-icon><Document /></el-icon>
                <span>{{ verifyForm.audioFile.name }}</span>
                <span class="file-size">({{ formatFileSize(verifyForm.audioFile.size) }})</span>
              </div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Ghi âm -->
        <el-tab-pane label="Ghi âm" name="record">
          <div class="record-section">
            <div class="record-status">
              <div v-if="!isVerifyRecording && !verifyRecordedBlob" class="record-ready">
                <el-icon size="48" color="var(--apple-primary)"><Microphone /></el-icon>
                <p>Nhấn nút bên dưới để bắt đầu ghi âm</p>
                <p class="record-tip">Khuyến nghị ghi âm 3-10 giây rõ ràng</p>
              </div>
              <div v-else-if="isVerifyRecording" class="record-recording">
                <div class="recording-indicator">
                  <span class="recording-dot"></span>
                  <span class="recording-text">Đang ghi âm...</span>
                </div>
                <div class="record-time">{{ formatRecordTime(verifyRecordTime) }}</div>
                <p class="record-tip">Nhấn nút dừng để kết thúc ghi âm</p>
              </div>
              <div v-else-if="verifyRecordedBlob" class="record-complete">
                <el-icon size="48" color="var(--apple-success)"><CircleCheck /></el-icon>
                <p>Ghi âm hoàn tất</p>
                <p class="record-tip">Thời lượng: {{ formatRecordTime(verifyRecordTime) }}</p>
                <audio :src="verifyRecordedBlobUrl" controls class="record-preview"></audio>
              </div>
            </div>

            <div class="record-controls">
              <el-button
                v-if="!isVerifyRecording && !verifyRecordedBlob"
                type="primary"
                size="large"
                @click="startVerifyRecording"
                :disabled="!canRecord"
              >
                <el-icon><VideoPlay /></el-icon>
                Bắt đầu ghi âm
              </el-button>
              <el-button v-if="isVerifyRecording" type="danger" size="large" @click="stopVerifyRecording">
                <el-icon><VideoPause /></el-icon>
                Dừng ghi âm
              </el-button>
              <el-button
                v-if="verifyRecordedBlob"
                type="primary"
                size="large"
                @click="startVerifyRecording"
                :disabled="!canRecord"
              >
                <el-icon><Refresh /></el-icon>
                Ghi lại
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <!-- Hiển thị kết quả xác minh -->
      <div v-if="verifyResult" class="verify-result">
        <el-divider>Kết quả xác minh</el-divider>
        <div :class="['result-content', verifyResult.verified ? 'result-success' : 'result-failed']">
          <div class="result-icon">
            <el-icon v-if="verifyResult.verified" size="48" color="var(--apple-success)"><CircleCheck /></el-icon>
            <el-icon v-else size="48" color="var(--apple-danger)"><CircleClose /></el-icon>
          </div>
          <div class="result-info">
            <div class="result-status">
              {{ verifyResult.verified ? 'Xác minh thành công' : 'Xác minh thất bại' }}
            </div>
            <div class="result-details">
              <div>
                Độ tin cậy: <strong>{{ (verifyResult.confidence * 100).toFixed(1) }}%</strong>
              </div>
              <div>Ngưỡng: {{ (verifyResult.threshold * 100).toFixed(1) }}%</div>
            </div>
            <div class="result-message">{{ verifyResult.message }}</div>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="handleCloseVerifyDialog">Hủy</el-button>
        <el-button type="primary" @click="handleSubmitVerify" :loading="verifying" :disabled="!hasVerifyAudioFile">
          Xác minh
        </el-button>
      </template>
    </el-dialog>

    <!-- Trình phát âm thanh (ẩn) -->
    <audio ref="audioPlayer" style="display: none" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  Plus,
  Edit,
  Delete,
  View,
  Search,
  UploadFilled,
  Document,
  DocumentCopy,
  VideoPlay,
  Download,
  Microphone,
  CircleCheck,
  CircleClose,
  Refresh,
  VideoPause,
} from '@element-plus/icons-vue';
import api from '../../utils/api';

const loading = ref(false);
const submitting = ref(false);
const speakerGroups = ref([]);
const agents = ref([]);
const samples = ref([]);
const filterAgentId = ref('');
const searchKeyword = ref('');

// Trạng thái hộp thoại
const showGroupDialog = ref(false);
const groupDialogMode = ref('add'); // 'add' | 'edit'
const currentGroup = ref(null);
const showSampleDrawer = ref(false);
const showUploadDialog = ref(false);
const uploadMode = ref('history'); // 'upload' | 'record' | 'history'

// Liên quan hộp thoại xác minh
const showVerifyDialog = ref(false);
const verifyMode = ref('upload'); // 'upload' | 'record'
const currentVerifyGroup = ref(null);
const verifying = ref(false);
const verifyResult = ref(null);

// Biểu mẫu xác minh
const verifyForm = reactive({
  audioFile: null,
  audio: null,
});

// Danh sách tệp xác minh (dùng cho el-upload)
const verifyFileList = ref([]);

const verifyRules = {
  audio: [
    {
      validator: (rule, value, callback) => {
        if (!verifyForm.audioFile && !verifyRecordedBlob.value) {
          callback(new Error('Vui lòng tải lên hoặc ghi âm tệp âm thanh'));
        } else {
          callback();
        }
      },
      trigger: ['change', 'blur'],
    },
  ],
};

// Liên quan ghi âm xác minh
const isVerifyRecording = ref(false);
const verifyMediaRecorder = ref(null);
const verifyRecordedBlob = ref(null);
const verifyRecordedBlobUrl = ref('');
const verifyRecordTime = ref(0);
const verifyRecordTimer = ref(null);

// Liên quan ghi âm
const isRecording = ref(false);
const mediaRecorder = ref(null);
const recordedBlob = ref(null);
const recordedBlobUrl = ref('');
const recordTime = ref(0);
const recordTimer = ref(null);
const canRecord = ref(false);

// Tham chiếu biểu mẫu
const groupFormRef = ref();
const uploadFormRef = ref();
const uploadRef = ref();
const verifyFormRef = ref();
const verifyUploadRef = ref();
const audioPlayer = ref();

// Biểu mẫu nhóm giọng nói
const groupForm = reactive({
  agent_id: null,
  name: '',
  prompt: '',
  description: '',
  tts_config_id: null,
  voice: null,
});

const groupRules = {
  agent_id: [{ required: true, message: 'Vui lòng chọn trợ lý AI liên kết', trigger: 'change' }],
  name: [
    { required: true, message: 'Vui lòng nhập tên giọng nói', trigger: 'blur' },
    { min: 1, max: 100, message: 'Độ dài từ 1 đến 100 ký tự', trigger: 'blur' },
  ],
};

// Liên quan cấu hình TTS
const ttsConfigs = ref([]);
const currentVoiceOptions = ref([]);
const cloneVoicePresets = ref([]);
const cloneVoicesLoading = ref(false);

// Biểu mẫu tải lên
const uploadForm = reactive({
  audioFile: null,
  audio: null,
});

const uploadRules = {
  audio: [
    {
      validator: (rule, value, callback) => {
        if (!uploadForm.audioFile && !recordedBlob.value) {
          callback(new Error('Vui lòng tải lên hoặc ghi âm tệp âm thanh'));
        } else {
          callback();
        }
      },
      trigger: ['change', 'blur'],
    },
  ],
};

// Liên quan lịch sử
const loadingHistory = ref(false);
const historyMessages = ref([]);
const historyForm = reactive({
  agent_id: null,
  selected_message_id: null,
});

// Tính xem có tệp âm thanh không
const hasAudioFile = computed(() => {
  if (uploadMode.value === 'history') {
    return historyForm.selected_message_id !== null;
  }
  return uploadForm.audioFile !== null || recordedBlob.value !== null;
});

// Danh sách nhóm giọng nói đã lọc
const filteredGroups = computed(() => {
  let result = speakerGroups.value;

  // Lọc theo trợ lý AI
  if (filterAgentId.value) {
    result = result.filter((g) => g.agent_id === filterAgentId.value);
  }

  // Tìm kiếm theo từ khóa
  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase();
    result = result.filter(
      (g) =>
        g.name.toLowerCase().includes(keyword) ||
        (g.prompt && g.prompt.toLowerCase().includes(keyword)) ||
        (g.description && g.description.toLowerCase().includes(keyword)),
    );
  }

  return result;
});

// Tải danh sách trợ lý AI
const loadAgents = async () => {
  try {
    const response = await api.get('/user/agents');
    agents.value = response.data.data || [];
  } catch (error) {
    console.error('Tải danh sách trợ lý AI thất bại:', error);
    ElMessage.error('Tải danh sách trợ lý AI thất bại');
  }
};

// Tải danh sách cấu hình TTS
const loadTtsConfigs = async () => {
  try {
    const response = await api.get('/user/tts-configs');
    ttsConfigs.value = response.data.data || [];
  } catch (error) {
    console.error('Tải cấu hình TTS thất bại:', error);
    ElMessage.error('Tải cấu hình TTS thất bại');
  }
};

const normalizeCloneStatus = (clone) => {
  const status = String(clone?.status || '')
    .trim()
    .toLowerCase();
  const taskStatus = String(clone?.task_status || '')
    .trim()
    .toLowerCase();
  if (status === 'failed' || taskStatus === 'failed') return 'failed';
  if (status === 'active' || taskStatus === 'succeeded') return 'active';
  if (taskStatus === 'queued' || taskStatus === 'processing') return taskStatus;
  if (status === 'queued' || status === 'processing') return status;
  return status || taskStatus || 'unknown';
};

const loadCloneVoicePresets = async () => {
  cloneVoicesLoading.value = true;
  try {
    const response = await api.get('/user/voice-clones');
    const cloneList = response.data.data || [];
    cloneVoicePresets.value = cloneList
      .filter((clone) => normalizeCloneStatus(clone) === 'active')
      .filter((clone) => clone?.tts_config_id && clone?.provider_voice_id)
      .map((clone) => ({
        id: clone.id,
        name: clone.name || clone.provider_voice_id,
        provider_voice_id: clone.provider_voice_id,
        tts_config_id: clone.tts_config_id,
        tts_config_name: clone.tts_config_name || '',
      }));
  } catch (error) {
    console.error('Tải giọng đọc sao chép thất bại:', error);
    cloneVoicePresets.value = [];
  } finally {
    cloneVoicesLoading.value = false;
  }
};

const isCloneVoiceSelected = (clone) => {
  return groupForm.tts_config_id === clone?.tts_config_id && groupForm.voice === clone?.provider_voice_id;
};

const applyCloneVoice = async (clone) => {
  if (!clone) return;
  const ttsConfig = ttsConfigs.value.find((config) => config.config_id === clone.tts_config_id);
  if (!ttsConfig) {
    return;
  }
  groupForm.tts_config_id = clone.tts_config_id;
  await handleTtsConfigChange(clone.tts_config_id);
  groupForm.voice = clone.provider_voice_id;
};

// Khi cấu hình TTS thay đổi, tải danh sách giọng đọc tương ứng
const handleTtsConfigChange = async (configId) => {
  if (!configId) {
    currentVoiceOptions.value = [];
    groupForm.voice = null;
    return;
  }

  const config = ttsConfigs.value.find((c) => c.config_id === configId);
  if (!config) {
    currentVoiceOptions.value = [];
    return;
  }

  try {
    // Lấy danh sách giọng đọc đầy đủ từ backend API
    const params = { provider: config.provider };
    // Luôn đính kèm tham số config_id
    if (configId) {
      params.config_id = configId;
    }
    const response = await api.get('/user/voice-options', { params });
    currentVoiceOptions.value = response.data.data || [];
  } catch (error) {
    console.error('Tải danh sách giọng đọc thất bại:', error);
    currentVoiceOptions.value = [];
    ElMessage.warning('Tải danh sách giọng đọc thất bại, vui lòng thử lại sau');
  }
};

// Trích xuất tùy chọn giọng đọc theo provider
const extractVoiceOptions = (provider, config) => {
  const options = [];

  if (!config) return options;

  // Trích xuất giọng đọc theo nhà cung cấp TTS
  switch (provider) {
    case 'edge':
    case 'microsoft':
      // Giọng phổ biến Edge TTS
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice });
      }
      // Thêm giọng phổ biến
      const edgeVoices = [
        { label: 'zh-CN-XiaoxiaoNeural (Tiểu Tiểu)', value: 'zh-CN-XiaoxiaoNeural' },
        { label: 'zh-CN-YunxiNeural (Vân Hi)', value: 'zh-CN-YunxiNeural' },
        { label: 'zh-CN-YunyangNeural (Vân Dương)', value: 'zh-CN-YunyangNeural' },
        { label: 'zh-CN-XiaoyiNeural (Tiểu Y)', value: 'zh-CN-XiaoyiNeural' },
        { label: 'zh-CN-YunjianNeural (Vân Kiện)', value: 'zh-CN-YunjianNeural' },
        { label: 'zh-CN-XiaochenNeural (Tiểu Thần)', value: 'zh-CN-XiaochenNeural' },
        { label: 'zh-CN-XiaohanNeural (Tiểu Hàm)', value: 'zh-CN-XiaohanNeural' },
      ];
      edgeVoices.forEach((v) => {
        if (!options.find((o) => o.value === v.value)) {
          options.push(v);
        }
      });
      break;

    case 'doubao':
    case 'doubao_ws':
      // Giọng Doubao TTS
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice });
      }
      const doubaoVoices = [
        { label: 'Shuangkuaisisi (Giọng nữ ngọt ngào)', value: 'zh_female_shuangkuaisisi_moon_bigtts' },
        { label: 'BV700 V2 (Giọng nam)', value: 'BV700_V2_streaming' },
        { label: 'BV001 (Giọng nữ)', value: 'BV001_streaming' },
        { label: 'BV002 (Giọng nam)', value: 'BV002_streaming' },
      ];
      doubaoVoices.forEach((v) => {
        if (!options.find((o) => o.value === v.value)) {
          options.push(v);
        }
      });
      break;

    case 'cosyvoice':
      // CosyVoice dùng spk_id
      if (config.spk_id) {
        options.push({ label: config.spk_id, value: config.spk_id });
      }
      const cosyVoices = [
        { label: 'Nữ tiếng Trung', value: '中文女' },
        { label: 'Nam tiếng Trung', value: '中文男' },
        { label: 'Nữ tiếng Quảng', value: '粤语女' },
        { label: 'Nữ tiếng Anh', value: '英文女' },
        { label: 'Nam tiếng Anh', value: '英文男' },
        { label: 'Nam tiếng Nhật', value: '日语男' },
        { label: 'Nữ tiếng Hàn', value: '韩语女' },
      ];
      cosyVoices.forEach((v) => {
        if (!options.find((o) => o.value === v.value)) {
          options.push(v);
        }
      });
      break;

    case 'minimax':
      // Minimax TTS dùng voice
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice });
      }
      const minimaxVoices = [
        { label: 'Xanh non (Nam)', value: 'male-qn-qingse' },
        { label: 'Xanh non (Nữ)', value: 'female-qn-qingse' },
        { label: 'Thiếu niên (Nam)', value: 'male-shaonian' },
        { label: 'Thiếu nữ (Nữ)', value: 'female-shaonian' },
        { label: 'Trưởng thành (Nam)', value: 'male-chengshu' },
        { label: 'Trưởng thành (Nữ)', value: 'female-chengshu' },
        { label: 'Ấm áp (Nam)', value: 'male-wennuan' },
        { label: 'Ấm áp (Nữ)', value: 'female-wennuan' },
        { label: 'Trong sáng (Nam)', value: 'male-qinglang' },
        { label: 'Trong sáng (Nữ)', value: 'female-qinglang' },
        { label: 'Trầm ấm (Nam)', value: 'male-houzhong' },
        { label: 'Trầm ấm (Nữ)', value: 'female-houzhong' },
      ];
      minimaxVoices.forEach((v) => {
        if (!options.find((o) => o.value === v.value)) {
          options.push(v);
        }
      });
      break;

    default:
      // Provider khác, thử trích xuất từ cấu hình
      if (config.voice) {
        options.push({ label: config.voice, value: config.voice });
      }
      if (config.spk_id) {
        options.push({ label: config.spk_id, value: config.spk_id });
      }
  }

  return options;
};

// Lấy tên cấu hình TTS hiện tại
const getCurrentTtsConfigName = () => {
  if (!groupForm.tts_config_id) return '';
  const config = ttsConfigs.value.find((c) => c.config_id === groupForm.tts_config_id);
  return config ? config.name : '';
};

// Lấy thông tin cấu hình TTS hiện tại
const getCurrentTtsConfigInfo = () => {
  if (!groupForm.tts_config_id) return '';
  const config = ttsConfigs.value.find((c) => c.config_id === groupForm.tts_config_id);
  if (!config) return '';
  return `Nhà cung cấp TTS: ${config.provider || 'Không rõ'}`;
};

// Tải danh sách nhóm giọng nói
const loadSpeakerGroups = async () => {
  try {
    loading.value = true;
    const params = {};
    if (filterAgentId.value) {
      params.agent_id = filterAgentId.value;
    }
    const response = await api.get('/user/speaker-groups', { params });
    speakerGroups.value = response.data.data || [];
  } catch (error) {
    console.error('Tải danh sách nhóm giọng nói thất bại:', error);
    ElMessage.error('Tải danh sách nhóm giọng nói thất bại: ' + (error.response?.data?.error || error.message));
  } finally {
    loading.value = false;
  }
};

// Xử lý tìm kiếm
const handleSearch = () => {
  // Tìm kiếm được lọc phía client, không cần gửi lại yêu cầu
};

// Tạo nhóm giọng nói
const handleAddGroup = async () => {
  groupDialogMode.value = 'add';
  resetGroupForm();
  await loadCloneVoicePresets();
  showGroupDialog.value = true;
};

// Chỉnh sửa nhóm giọng nói
const handleEditGroup = async (group) => {
  groupDialogMode.value = 'edit';
  currentGroup.value = group;
  groupForm.agent_id = group.agent_id;
  groupForm.name = group.name;
  groupForm.prompt = group.prompt || '';
  groupForm.description = group.description || '';
  groupForm.tts_config_id = group.tts_config_id || null;
  groupForm.voice = group.voice || null;
  await loadCloneVoicePresets();

  // Nếu có cấu hình TTS, tải danh sách giọng đọc tương ứng
  if (groupForm.tts_config_id) {
    await handleTtsConfigChange(groupForm.tts_config_id);
  }

  showGroupDialog.value = true;
};

// Gửi nhóm giọng nói
const handleSubmitGroup = async () => {
  if (!groupFormRef.value) return;

  try {
    await groupFormRef.value.validate();
    submitting.value = true;

    if (groupDialogMode.value === 'add') {
      const response = await api.post('/user/speaker-groups', groupForm);
      ElMessage.success('Tạo thành công');
      showGroupDialog.value = false;
      await loadSpeakerGroups();
    } else {
      const response = await api.put(`/user/speaker-groups/${currentGroup.value.id}`, groupForm);
      ElMessage.success('Cập nhật thành công');
      showGroupDialog.value = false;
      await loadSpeakerGroups();
    }
  } catch (error) {
    if (error.fields) {
      // Lỗi xác thực biểu mẫu
      return;
    }
    console.error('Gửi thất bại:', error);
    ElMessage.error('Thao tác thất bại: ' + (error.response?.data?.error || error.message));
  } finally {
    submitting.value = false;
  }
};

// Xác minh nhóm giọng nói
const handleVerifyGroup = async (group) => {
  // Dọn dữ liệu trước
  resetVerifyForm();

  // Chờ DOM cập nhật xong
  await nextTick();

  currentVerifyGroup.value = group;
  verifyResult.value = null;
  verifyMode.value = 'upload';
  showVerifyDialog.value = true;

  // Đảm bảo xóa component tải lên lần nữa
  await nextTick();
  verifyUploadRef.value?.clearFiles();
  verifyFileList.value = [];

  // Kiểm tra trình duyệt có hỗ trợ ghi âm không
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    stream.getTracks().forEach((track) => track.stop());
    canRecord.value = true;
  } catch (error) {
    console.warn('Trình duyệt không hỗ trợ ghi âm:', error);
    canRecord.value = false;
    if (verifyMode.value === 'record') {
      ElMessage.warning('Trình duyệt của bạn không hỗ trợ ghi âm, vui lòng tải lên tệp');
      verifyMode.value = 'upload';
    }
  }
};

// Đóng hộp thoại xác minh
const handleCloseVerifyDialog = () => {
  if (isVerifyRecording.value) {
    stopVerifyRecording();
  }
  resetVerifyForm();
  showVerifyDialog.value = false;
};

// Xử lý thay đổi tệp xác minh
const handleVerifyFileChange = async (file, fileList) => {
  // Xóa danh sách tệp trước, đảm bảo tệp cũ bị gỡ
  verifyFileList.value = [];
  await nextTick();

  // Nếu đã có tệp, dọn dẹp tệp trước
  if (verifyForm.audioFile) {
    verifyForm.audioFile = null;
    verifyForm.audio = null;
  }

  // Dọn dẹp liên quan ghi âm
  if (verifyRecordedBlob.value) {
    if (verifyRecordedBlobUrl.value) {
      URL.revokeObjectURL(verifyRecordedBlobUrl.value);
      verifyRecordedBlobUrl.value = '';
    }
    verifyRecordedBlob.value = null;
    verifyRecordTime.value = 0;
  }

  // Dọn dẹp kết quả xác minh
  verifyResult.value = null;

  const fileObj = file.raw || file;
  if (!fileObj) {
    ElMessage.warning('Đối tượng tệp không hợp lệ');
    verifyUploadRef.value?.clearFiles();
    verifyForm.audioFile = null;
    verifyFileList.value = [];
    return;
  }

  // Kiểm tra loại tệp
  const fileName = fileObj.name || file.name || '';
  const fileType = fileObj.type || file.type || '';
  if (!fileType.includes('wav') && !fileName.toLowerCase().endsWith('.wav')) {
    ElMessage.warning('Chỉ hỗ trợ định dạng WAV');
    verifyUploadRef.value?.clearFiles();
    verifyForm.audioFile = null;
    verifyFileList.value = [];
    return;
  }

  // Kiểm tra kích thước tệp (10MB)
  const fileSize = fileObj.size || file.size || 0;
  if (fileSize > 10 * 1024 * 1024) {
    ElMessage.warning('Kích thước tệp không được vượt quá 10MB');
    verifyUploadRef.value?.clearFiles();
    verifyForm.audioFile = null;
    verifyFileList.value = [];
    return;
  }

  // Đặt tệp mới
  verifyForm.audioFile = file;
  verifyForm.audio = file;

  // Cập nhật danh sách tệp hiển thị (chỉ hiện tệp mới nhất)
  verifyFileList.value = [file];

  await nextTick();

  if (verifyFormRef.value) {
    verifyFormRef.value.clearValidate('audio');
  }
};

// Xử lý gỡ tệp xác minh
const handleVerifyFileRemove = () => {
  verifyForm.audioFile = null;
  verifyForm.audio = null;
  verifyFileList.value = [];
  verifyResult.value = null; // Dọn dẹp kết quả xác minh
  if (verifyFormRef.value) {
    verifyFormRef.value.validateField('audio');
  }
};

// Bắt đầu ghi âm xác minh
const startVerifyRecording = async () => {
  try {
    // Dừng ghi âm trước (nếu có)
    if (verifyMediaRecorder.value && verifyMediaRecorder.value.state !== 'inactive') {
      verifyMediaRecorder.value.stop();
    }

    // Dọn dẹp ghi âm trước
    if (verifyRecordedBlobUrl.value) {
      URL.revokeObjectURL(verifyRecordedBlobUrl.value);
      verifyRecordedBlobUrl.value = '';
    }
    verifyRecordedBlob.value = null;
    verifyRecordTime.value = 0;

    // Lấy quyền microphone
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: 16000,
        echoCancellation: true,
        noiseSuppression: true,
      },
    });

    // Tạo MediaRecorder
    const chunks = [];
    const options = {
      mimeType: 'audio/webm;codecs=opus',
    };

    if (!MediaRecorder.isTypeSupported(options.mimeType)) {
      verifyMediaRecorder.value = new MediaRecorder(stream);
    } else {
      verifyMediaRecorder.value = new MediaRecorder(stream, options);
    }

    verifyMediaRecorder.value.ondataavailable = (e) => {
      if (e.data.size > 0) {
        chunks.push(e.data);
      }
    };

    verifyMediaRecorder.value.onstop = async () => {
      stream.getTracks().forEach((track) => track.stop());

      try {
        // Chuyển đổi âm thanh sang định dạng WAV
        const blob = new Blob(chunks, { type: chunks[0]?.type || 'audio/webm' });
        const wavBlob = await convertToWav(blob);

        verifyRecordedBlob.value = wavBlob;
        verifyRecordedBlobUrl.value = URL.createObjectURL(wavBlob);

        // Tạo đối tượng File để tải lên
        const fileName = `verify_recording_${Date.now()}.wav`;
        const file = new File([wavBlob], fileName, { type: 'audio/wav' });
        verifyForm.audioFile = { raw: file, name: fileName, size: wavBlob.size };
        verifyForm.audio = file;

        if (verifyFormRef.value) {
          verifyFormRef.value.clearValidate('audio');
        }
      } catch (error) {
        console.error('Xử lý dữ liệu ghi âm thất bại:', error);
        ElMessage.error('Xử lý dữ liệu ghi âm thất bại, vui lòng thử lại');
        verifyRecordedBlob.value = null;
        verifyRecordedBlobUrl.value = '';
        verifyForm.audioFile = null;
        verifyForm.audio = null;
      }

      chunks.length = 0;
    };

    // Bắt đầu ghi
    verifyMediaRecorder.value.start(100);
    isVerifyRecording.value = true;

    // Bắt đầu đếm giờ
    verifyRecordTimer.value = setInterval(() => {
      verifyRecordTime.value += 0.1;
    }, 100);

    ElMessage.success('Bắt đầu ghi âm');
  } catch (error) {
    console.error('Ghi âm thất bại:', error);
    ElMessage.error('Ghi âm thất bại: ' + error.message);
    canRecord.value = false;
  }
};

// Dừng ghi âm xác minh
const stopVerifyRecording = () => {
  if (verifyMediaRecorder.value && verifyMediaRecorder.value.state !== 'inactive') {
    verifyMediaRecorder.value.stop();
  }
  isVerifyRecording.value = false;

  if (verifyRecordTimer.value) {
    clearInterval(verifyRecordTimer.value);
    verifyRecordTimer.value = null;
  }

  ElMessage.success('Ghi âm hoàn tất');
};

// Gửi xác minh
const handleSubmitVerify = async () => {
  if (!verifyFormRef.value) return;

  try {
    await verifyFormRef.value.validate();

    if (!verifyForm.audioFile && !verifyRecordedBlob.value) {
      ElMessage.warning('Vui lòng tải lên hoặc ghi âm tệp âm thanh');
      return;
    }

    verifying.value = true;
    verifyResult.value = null;

    let file;
    if (verifyForm.audioFile) {
      // Dùng tệp đã tải lên
      file = verifyForm.audioFile.raw || verifyForm.audioFile;
    } else if (verifyRecordedBlob.value) {
      // Dùng âm thanh đã ghi
      const fileName = `verify_recording_${Date.now()}.wav`;
      file = new File([verifyRecordedBlob.value], fileName, { type: 'audio/wav' });
    } else {
      ElMessage.warning('Vui lòng tải lên hoặc ghi âm tệp âm thanh');
      return;
    }

    const formData = new FormData();
    formData.append('audio', file);

    const response = await api.post(`/user/speaker-groups/${currentVerifyGroup.value.id}/verify`, formData);

    if (response.data.success && response.data.data) {
      verifyResult.value = {
        verified: response.data.data.verified,
        confidence: response.data.data.confidence,
        threshold: response.data.data.threshold,
        message: response.data.data.message,
      };

      if (verifyResult.value.verified) {
        ElMessage.success('Xác minh thành công!');
      } else {
        ElMessage.warning('Xác minh không thành công');
      }
    } else {
      ElMessage.error('Xác minh thất bại');
    }
  } catch (error) {
    if (error.fields) {
      return;
    }
    console.error('Xác minh thất bại:', error);
    ElMessage.error('Xác minh thất bại: ' + (error.response?.data?.error || error.message));
  } finally {
    verifying.value = false;
  }
};

// Đặt lại biểu mẫu xác minh
const resetVerifyForm = () => {
  if (verifyFormRef.value) {
    verifyFormRef.value.resetFields();
  }
  if (verifyUploadRef.value) {
    verifyUploadRef.value.clearFiles();
  }
  verifyForm.audioFile = null;
  verifyForm.audio = null;

  // Dọn dẹp liên quan ghi âm xác minh
  if (isVerifyRecording.value) {
    stopVerifyRecording();
  }
  if (verifyRecordedBlobUrl.value) {
    URL.revokeObjectURL(verifyRecordedBlobUrl.value);
    verifyRecordedBlobUrl.value = '';
  }
  verifyRecordedBlob.value = null;
  verifyRecordTime.value = 0;
  verifyMode.value = 'upload';
  verifyResult.value = null;
};

// Kiểm tra có tệp âm thanh xác minh không
const hasVerifyAudioFile = computed(() => {
  return verifyForm.audioFile !== null || verifyRecordedBlob.value !== null;
});

// Xóa nhóm giọng nói
const handleDeleteGroup = async (group) => {
  try {
    await ElMessageBox.confirm(
      `Bạn có chắc muốn xóa nhóm giọng nói "${group.name}" không? Thao tác này sẽ xóa toàn bộ mẫu trong nhóm và không thể phục hồi.`,
      'Xác nhận xóa',
      {
        confirmButtonText: 'Xác nhận',
        cancelButtonText: 'Hủy',
        type: 'warning',
      },
    );

    loading.value = true;
    await api.delete(`/user/speaker-groups/${group.id}`);
    ElMessage.success('Xóa thành công');
    await loadSpeakerGroups();
  } catch (error) {
    if (error !== 'cancel') {
      console.error('Xóa thất bại:', error);
      ElMessage.error('Xóa thất bại: ' + (error.response?.data?.error || error.message));
    }
  } finally {
    loading.value = false;
  }
};

// Xem mẫu
const handleViewSamples = async (group) => {
  currentGroup.value = group;
  showSampleDrawer.value = true;
  await loadSamples(group.id);
};

// Xác minh nhóm giọng nói từ ngăn kéo quản lý mẫu
const handleVerifyFromSamples = () => {
  if (currentGroup.value) {
    showSampleDrawer.value = false;
    handleVerifyGroup(currentGroup.value);
  }
};

// Tải danh sách mẫu
const loadSamples = async (groupId) => {
  try {
    const response = await api.get(`/user/speaker-groups/${groupId}/samples`);
    samples.value = response.data.data || [];
  } catch (error) {
    console.error('Tải danh sách mẫu thất bại:', error);
    ElMessage.error('Tải danh sách mẫu thất bại');
  }
};

// Đóng ngăn kéo mẫu
const handleCloseSampleDrawer = () => {
  showSampleDrawer.value = false;
  currentGroup.value = null;
  samples.value = [];
};

// Thêm mẫu
const handleAddSample = async () => {
  resetUploadForm();
  uploadMode.value = 'history';
  showUploadDialog.value = true;

  // Khởi tạo biểu mẫu lịch sử
  historyForm.agent_id = currentGroup.value?.agent_id || null;
  historyForm.selected_message_id = null;
  historyMessages.value = [];

  // Nếu nhóm giọng nói có trợ lý AI liên kết, tự động tải lịch sử
  if (currentGroup.value?.agent_id) {
    historyForm.agent_id = currentGroup.value.agent_id;
    await loadHistoryMessages();
  }

  // Kiểm tra trình duyệt có hỗ trợ ghi âm không
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    stream.getTracks().forEach((track) => track.stop());
    canRecord.value = true;
  } catch (error) {
    console.warn('Trình duyệt không hỗ trợ ghi âm:', error);
    canRecord.value = false;
    if (uploadMode.value === 'record') {
      ElMessage.warning('Trình duyệt của bạn không hỗ trợ ghi âm, vui lòng tải lên tệp');
      uploadMode.value = 'upload';
    }
  }
};

// Đóng hộp thoại tải lên
const handleCloseUploadDialog = () => {
  if (isRecording.value) {
    stopRecording();
  }
  resetUploadForm();
  showUploadDialog.value = false;
};

// Xử lý thay đổi tệp
const handleFileChange = (file) => {
  const fileObj = file.raw || file;
  if (!fileObj) {
    ElMessage.warning('Đối tượng tệp không hợp lệ');
    uploadRef.value?.clearFiles();
    uploadForm.audioFile = null;
    return;
  }

  // Kiểm tra loại tệp
  const fileName = fileObj.name || file.name || '';
  const fileType = fileObj.type || file.type || '';
  if (!fileType.includes('wav') && !fileName.toLowerCase().endsWith('.wav')) {
    ElMessage.warning('Chỉ hỗ trợ định dạng WAV');
    uploadRef.value?.clearFiles();
    uploadForm.audioFile = null;
    return;
  }

  // Kiểm tra kích thước tệp (10MB)
  const fileSize = fileObj.size || file.size || 0;
  if (fileSize > 10 * 1024 * 1024) {
    ElMessage.warning('Kích thước tệp không được vượt quá 10MB');
    uploadRef.value?.clearFiles();
    uploadForm.audioFile = null;
    return;
  }

  uploadForm.audioFile = file;
  uploadForm.audio = file;

  if (uploadFormRef.value) {
    uploadFormRef.value.clearValidate('audio');
  }
};

// Xử lý gỡ tệp
const handleFileRemove = () => {
  uploadForm.audioFile = null;
  uploadForm.audio = null;
  if (uploadFormRef.value) {
    uploadFormRef.value.validateField('audio');
  }
};

// Bắt đầu ghi âm
const startRecording = async () => {
  try {
    // Dừng ghi âm trước (nếu có)
    if (mediaRecorder.value && mediaRecorder.value.state !== 'inactive') {
      mediaRecorder.value.stop();
    }

    // Dọn dẹp ghi âm trước
    if (recordedBlobUrl.value) {
      URL.revokeObjectURL(recordedBlobUrl.value);
      recordedBlobUrl.value = '';
    }
    recordedBlob.value = null;
    recordTime.value = 0;

    // Lấy quyền microphone
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        sampleRate: 16000,
        echoCancellation: true,
        noiseSuppression: true,
      },
    });

    // Tạo MediaRecorder (định dạng WAV)
    const chunks = [];
    const options = {
      mimeType: 'audio/webm;codecs=opus', // Ghi webm trước, sau đó chuyển sang WAV
    };

    // Kiểm tra hỗ trợ trình duyệt
    if (!MediaRecorder.isTypeSupported(options.mimeType)) {
      // Nếu không hỗ trợ, dùng định dạng mặc định
      mediaRecorder.value = new MediaRecorder(stream);
    } else {
      mediaRecorder.value = new MediaRecorder(stream, options);
    }

    mediaRecorder.value.ondataavailable = (e) => {
      if (e.data.size > 0) {
        chunks.push(e.data);
      }
    };

    mediaRecorder.value.onstop = async () => {
      stream.getTracks().forEach((track) => track.stop());

      try {
        // Chuyển đổi âm thanh sang định dạng WAV
        const blob = new Blob(chunks, { type: chunks[0]?.type || 'audio/webm' });
        const wavBlob = await convertToWav(blob);

        recordedBlob.value = wavBlob;
        recordedBlobUrl.value = URL.createObjectURL(wavBlob);

        // Tạo đối tượng File để tải lên
        const fileName = `recording_${Date.now()}.wav`;
        const file = new File([wavBlob], fileName, { type: 'audio/wav' });
        uploadForm.audioFile = { raw: file, name: fileName, size: wavBlob.size };
        uploadForm.audio = file;

        if (uploadFormRef.value) {
          uploadFormRef.value.clearValidate('audio');
        }
      } catch (error) {
        console.error('Xử lý dữ liệu ghi âm thất bại:', error);
        ElMessage.error('Xử lý dữ liệu ghi âm thất bại, vui lòng thử lại');
        recordedBlob.value = null;
        recordedBlobUrl.value = '';
        uploadForm.audioFile = null;
        uploadForm.audio = null;
      }

      chunks.length = 0;
    };

    // Bắt đầu ghi
    mediaRecorder.value.start(100); // Thu thập mỗi 100ms
    isRecording.value = true;

    // Bắt đầu đếm giờ
    recordTimer.value = setInterval(() => {
      recordTime.value += 0.1;
    }, 100);

    ElMessage.success('Bắt đầu ghi âm');
  } catch (error) {
    console.error('Ghi âm thất bại:', error);
    ElMessage.error('Ghi âm thất bại: ' + error.message);
    canRecord.value = false;
  }
};

// Dừng ghi âm
const stopRecording = () => {
  if (mediaRecorder.value && mediaRecorder.value.state !== 'inactive') {
    mediaRecorder.value.stop();
  }
  isRecording.value = false;

  if (recordTimer.value) {
    clearInterval(recordTimer.value);
    recordTimer.value = null;
  }

  ElMessage.success('Ghi âm hoàn tất');
};

// Chuyển đổi âm thanh sang định dạng WAV
const convertToWav = async (blob) => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = async (e) => {
      try {
        const audioContext = new (window.AudioContext || window.webkitAudioContext)();
        const arrayBuffer = e.target.result;
        const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);

        // Chuyển sang WAV
        const wav = audioBufferToWav(audioBuffer);
        const wavBlob = new Blob([wav], { type: 'audio/wav' });
        resolve(wavBlob);
      } catch (error) {
        console.error('Chuyển đổi WAV thất bại:', error);
        // Nếu chuyển đổi thất bại, dùng blob gốc (backend có thể cần hỗ trợ webm)
        reject(error);
      }
    };
    reader.onerror = reject;
    reader.readAsArrayBuffer(blob);
  });
};

// Chuyển đổi AudioBuffer sang định dạng WAV
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

  // Header tệp WAV
  const writeString = (offset, string) => {
    for (let i = 0; i < string.length; i++) {
      view.setUint8(offset + i, string.charCodeAt(i));
    }
  };

  writeString(0, 'RIFF');
  view.setUint32(4, bufferSize - 8, true);
  writeString(8, 'WAVE');
  writeString(12, 'fmt ');
  view.setUint32(16, 16, true); // fmt chunk size
  view.setUint16(20, 1, true); // audio format (PCM)
  view.setUint16(22, numberOfChannels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, 16, true); // bits per sample
  writeString(36, 'data');
  view.setUint32(40, dataSize, true);

  // Ghi dữ liệu âm thanh
  let offset = 44;
  for (let i = 0; i < length; i++) {
    for (let channel = 0; channel < numberOfChannels; channel++) {
      const sample = Math.max(-1, Math.min(1, buffer.getChannelData(channel)[i]));
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
      offset += 2;
    }
  }

  return arrayBuffer;
};

// Định dạng thời lượng ghi âm
const formatRecordTime = (seconds) => {
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  const ms = Math.floor((seconds % 1) * 10);
  return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}.${ms}`;
};

// Tải lịch sử trò chuyện
const loadHistoryMessages = async () => {
  if (!historyForm.agent_id) {
    historyMessages.value = [];
    return;
  }

  try {
    loadingHistory.value = true;
    const response = await api.get('/user/history/messages', {
      params: {
        agent_id: historyForm.agent_id,
        role: 'user',
        page: 1,
        page_size: 50,
      },
    });

    // Chỉ hiển thị tin nhắn có âm thanh
    historyMessages.value = (response.data.data || []).filter((msg) => msg.audio_path);
  } catch (error) {
    console.error('Tải lịch sử trò chuyện thất bại:', error);
    ElMessage.error('Tải lịch sử trò chuyện thất bại: ' + (error.response?.data?.error || error.message));
    historyMessages.value = [];
  } finally {
    loadingHistory.value = false;
  }
};

// Chọn tin nhắn lịch sử
const handleSelectHistoryMessage = (row) => {
  historyForm.selected_message_id = row.message_id;
};

// Nghe thử âm thanh lịch sử
const handlePreviewHistoryAudio = async (message) => {
  try {
    const response = await api.get(`/user/history/messages/${message.id}/audio`, {
      responseType: 'blob',
    });

    const blob = new Blob([response.data], { type: 'audio/wav' });
    const blobUrl = URL.createObjectURL(blob);

    audioPlayer.value.src = blobUrl;
    audioPlayer.value.play().catch((err) => {
      console.error('Phát thất bại:', err);
      ElMessage.warning('Phát thất bại, vui lòng kiểm tra tệp âm thanh');
    });

    audioPlayer.value.onended = () => {
      URL.revokeObjectURL(blobUrl);
    };
  } catch (error) {
    console.error('Nghe thử thất bại:', error);
    ElMessage.error('Nghe thử thất bại: ' + (error.response?.data?.error || error.message));
  }
};

// Gửi mẫu
const handleSubmitSample = async () => {
  if (uploadMode.value === 'history') {
    // Chọn từ lịch sử
    if (!historyForm.selected_message_id) {
      ElMessage.warning('Vui lòng chọn một tin nhắn lịch sử');
      return;
    }

    try {
      submitting.value = true;
      const formData = new FormData();
      formData.append('message_id', historyForm.selected_message_id);

      await api.post(`/user/speaker-groups/${currentGroup.value.id}/samples`, formData);
      ElMessage.success('Thêm thành công');
      handleCloseUploadDialog();
      await loadSamples(currentGroup.value.id);
      await loadSpeakerGroups(); // Làm mới danh sách để cập nhật số mẫu
    } catch (error) {
      console.error('Thêm thất bại:', error);
      ElMessage.error('Thêm thất bại: ' + (error.response?.data?.error || error.message));
    } finally {
      submitting.value = false;
    }
    return;
  }

  // Logic tải lên/ghi âm ban đầu
  if (!uploadFormRef.value) return;

  try {
    await uploadFormRef.value.validate();

    if (!uploadForm.audioFile && !recordedBlob.value) {
      ElMessage.warning('Vui lòng tải lên hoặc ghi âm tệp âm thanh');
      return;
    }

    submitting.value = true;

    let file;
    if (uploadForm.audioFile) {
      // Dùng tệp đã tải lên
      file = uploadForm.audioFile.raw || uploadForm.audioFile;
    } else if (recordedBlob.value) {
      // Dùng âm thanh đã ghi
      const fileName = `recording_${Date.now()}.wav`;
      file = new File([recordedBlob.value], fileName, { type: 'audio/wav' });
    } else {
      ElMessage.warning('Vui lòng tải lên hoặc ghi âm tệp âm thanh');
      return;
    }

    const formData = new FormData();
    formData.append('audio', file);

    await api.post(`/user/speaker-groups/${currentGroup.value.id}/samples`, formData);
    ElMessage.success('Tải lên thành công');
    handleCloseUploadDialog();
    await loadSamples(currentGroup.value.id);
    await loadSpeakerGroups(); // Làm mới danh sách để cập nhật số mẫu
  } catch (error) {
    if (error.fields) {
      return;
    }
    console.error('Tải lên thất bại:', error);
    ElMessage.error('Tải lên thất bại: ' + (error.response?.data?.error || error.message));
  } finally {
    submitting.value = false;
  }
};

// Phát mẫu
const handlePlaySample = async (sample) => {
  try {
    // Xây dựng URL tệp âm thanh (backend cần cung cấp API truy cập tệp)
    // Dùng api.get lấy tệp, sau đó tạo blob URL
    const response = await api.get(`/user/speaker-groups/${currentGroup.value.id}/samples/${sample.id}/file`, {
      responseType: 'blob',
    });

    // Tạo blob URL
    const blob = new Blob([response.data], { type: 'audio/wav' });
    const blobUrl = URL.createObjectURL(blob);

    audioPlayer.value.src = blobUrl;
    audioPlayer.value.play().catch((err) => {
      console.error('Phát thất bại:', err);
      ElMessage.warning('Phát thất bại, vui lòng kiểm tra tệp âm thanh');
    });

    // Dọn dẹp blob URL sau khi phát xong
    audioPlayer.value.onended = () => {
      URL.revokeObjectURL(blobUrl);
    };
  } catch (error) {
    console.error('Phát thất bại:', error);
    ElMessage.error('Phát thất bại: ' + (error.response?.data?.error || error.message));
  }
};

// Tải xuống mẫu
const handleDownloadSample = async (sample) => {
  try {
    // Dùng api.get lấy tệp, sau đó tạo liên kết tải xuống
    const response = await api.get(`/user/speaker-groups/${currentGroup.value.id}/samples/${sample.id}/file`, {
      responseType: 'blob',
    });

    // Tạo blob URL và tải xuống
    const blob = new Blob([response.data], { type: 'audio/wav' });
    const blobUrl = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = blobUrl;
    link.download = sample.file_name || 'audio.wav';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    // Dọn dẹp blob URL
    setTimeout(() => {
      URL.revokeObjectURL(blobUrl);
    }, 100);
  } catch (error) {
    console.error('Tải xuống thất bại:', error);
    ElMessage.error('Tải xuống thất bại: ' + (error.response?.data?.error || error.message));
  }
};

// Xóa mẫu
const handleDeleteSample = async (sample) => {
  try {
    await ElMessageBox.confirm(
      `Bạn có chắc muốn xóa mẫu "${sample.file_name}" không? Thao tác này không thể phục hồi.`,
      'Xác nhận xóa',
      {
        confirmButtonText: 'Xác nhận',
        cancelButtonText: 'Hủy',
        type: 'warning',
      },
    );

    await api.delete(`/user/speaker-groups/${currentGroup.value.id}/samples/${sample.id}`);
    ElMessage.success('Xóa thành công');
    await loadSamples(currentGroup.value.id);
    await loadSpeakerGroups(); // Làm mới danh sách để cập nhật số mẫu
  } catch (error) {
    if (error !== 'cancel') {
      console.error('Xóa thất bại:', error);
      ElMessage.error('Xóa thất bại: ' + (error.response?.data?.error || error.message));
    }
  }
};

// Sao chép vào clipboard
const copyToClipboard = async (text) => {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success('Đã sao chép vào clipboard');
  } catch (error) {
    console.error('Sao chép thất bại:', error);
    ElMessage.error('Sao chép thất bại');
  }
};

// Đặt lại biểu mẫu
const resetGroupForm = () => {
  if (groupFormRef.value) {
    groupFormRef.value.resetFields();
  }
  Object.assign(groupForm, {
    agent_id: null,
    name: '',
    prompt: '',
    description: '',
    tts_config_id: null,
    voice: null,
  });
  currentGroup.value = null;
  currentVoiceOptions.value = [];
};

const resetUploadForm = () => {
  if (uploadFormRef.value) {
    uploadFormRef.value.resetFields();
  }
  if (uploadRef.value) {
    uploadRef.value.clearFiles();
  }
  uploadForm.audioFile = null;
  uploadForm.audio = null;

  // Dọn dẹp liên quan ghi âm
  if (isRecording.value) {
    stopRecording();
  }
  if (recordedBlobUrl.value) {
    URL.revokeObjectURL(recordedBlobUrl.value);
    recordedBlobUrl.value = '';
  }
  recordedBlob.value = null;
  recordTime.value = 0;
  uploadMode.value = 'history';

  // Dọn dẹp liên quan lịch sử
  historyForm.agent_id = null;
  historyForm.selected_message_id = null;
  historyMessages.value = [];
};

// Định dạng ngày
const formatDate = (dateString) => {
  if (!dateString) return '-';
  return new Date(dateString).toLocaleString('zh-CN');
};

// Rút gọn ID hiển thị
const truncateId = (id) => {
  if (!id) return '-';
  if (id.length > 20) {
    return id.substring(0, 10) + '...' + id.substring(id.length - 10);
  }
  return id;
};

// Rút gọn văn bản
const truncateText = (text, maxLength) => {
  if (!text) return '-';
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
};

// Định dạng kích thước tệp
const formatFileSize = (bytes) => {
  if (!bytes) return '0 B';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
};

onMounted(() => {
  loadAgents();
  loadSpeakerGroups();
  loadTtsConfigs();
  loadCloneVoicePresets();
});

// Dọn dẹp tài nguyên trước khi tháo component
onBeforeUnmount(() => {
  if (isRecording.value) {
    stopRecording();
  }
  if (recordedBlobUrl.value) {
    URL.revokeObjectURL(recordedBlobUrl.value);
  }
  if (recordTimer.value) {
    clearInterval(recordTimer.value);
  }
  if (mediaRecorder.value && mediaRecorder.value.state !== 'inactive') {
    mediaRecorder.value.stop();
  }
});
</script>

<style scoped>
.speakers-page {
  padding: 0;
}

.filter-bar {
  padding: 15px 20px;
  background: rgba(255, 255, 255, 0.88);
  border-radius: 8px;
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.create-group-button {
  margin-left: auto;
}

.speakers-content {
  background: rgba(255, 255, 255, 0.88);
  border-radius: 8px;
  padding: 20px;
}

.prompt-text {
  color: #606266;
  cursor: pointer;
}

.prompt-popover {
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.text-muted {
  color: #909399;
}

.uuid-text {
  font-family: monospace;
  font-size: 12px;
}

.empty-state {
  padding: 40px 0;
}

.sample-drawer {
  padding: 20px;
}

.group-info-card {
  margin-bottom: 20px;
}

.group-info h3 {
  margin: 0 0 15px 0;
  color: #303133;
}

.prompt-section,
.description-section {
  margin-top: 15px;
  padding-top: 15px;
  border-top: 1px solid #f0f0f0;
}

.prompt-section strong,
.description-section strong {
  display: block;
  margin-bottom: 8px;
  color: #606266;
}

.prompt-section p,
.description-section p {
  margin: 0;
  color: #303133;
  white-space: pre-wrap;
  word-break: break-word;
}

.samples-section {
  margin-top: 20px;
}

.samples-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.samples-header h4 {
  margin: 0;
  color: #303133;
}

.empty-samples {
  padding: 40px 0;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 8px 12px;
  background: rgba(248, 250, 252, 0.92);
  border: 1px solid rgba(229, 229, 234, 0.72);
  border-radius: 12px;
  font-size: 14px;
  color: var(--apple-text-secondary);
}

.file-size {
  color: #909399;
  font-size: 12px;
}

.clone-voice-line {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  width: 100%;
}

.clone-voice-item {
  display: inline-flex;
  align-items: center;
  max-width: 220px;
  min-width: 0;
  padding: 4px 10px;
  border: 1px solid #d1d5db;
  border-radius: 999px;
  background: #f8fafc;
  color: #374151;
  cursor: pointer;
  transition: all 0.2s ease;
  line-height: 1.2;
  outline: none;
}

.clone-voice-item:hover {
  border-color: #93c5fd;
  background: #f1f7ff;
}

.clone-voice-item.active {
  border-color: #3b82f6;
  background: #e9f2ff;
  color: #1d4ed8;
  box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.1);
}

.clone-voice-name {
  font-size: 12px;
  font-weight: 500;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.el-upload-dragger) {
  width: 100%;
}

.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.action-buttons .el-button {
  margin: 0;
  white-space: nowrap;
}

/* Kiểu hộp thoại tải lên */
.upload-tabs {
  margin-top: 10px;
}

.audio-upload {
  width: 100%;
}

.audio-upload :deep(.el-upload-dragger) {
  width: 100%;
  padding: 40px 20px;
}

.audio-upload :deep(.el-icon--upload) {
  font-size: 48px;
  color: var(--apple-primary);
  margin-bottom: 16px;
}

.audio-upload :deep(.el-upload__text) {
  font-size: 14px;
  color: #606266;
}

.audio-upload :deep(.el-upload__text em) {
  color: var(--apple-primary);
  font-style: normal;
}

.audio-upload :deep(.el-upload__tip) {
  margin-top: 12px;
  font-size: 12px;
  color: #909399;
}

/* Kiểu khu vực ghi âm */
.record-section {
  padding: 20px 0;
}

.record-status {
  min-height: 200px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 30px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 20px;
}

.record-ready,
.record-complete {
  text-align: center;
}

.record-ready p,
.record-complete p {
  margin: 12px 0 0 0;
  color: #303133;
  font-size: 16px;
}

.record-tip {
  margin-top: 8px !important;
  font-size: 14px !important;
  color: #909399 !important;
}

.record-recording {
  text-align: center;
}

.recording-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-bottom: 16px;
}

.recording-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--apple-danger);
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.5;
    transform: scale(1.2);
  }
}

.recording-text {
  font-size: 16px;
  color: var(--apple-danger);
  font-weight: 500;
}

.record-time {
  font-size: 32px;
  font-weight: 600;
  color: #303133;
  font-family: 'Courier New', monospace;
  margin: 20px 0;
}

.record-preview {
  width: 100%;
  max-width: 400px;
  margin-top: 20px;
}

.record-controls {
  display: flex;
  justify-content: center;
  gap: 12px;
}

.record-controls .el-button {
  min-width: 120px;
}

/* Kiểu khu vực lịch sử */
.history-section {
  padding: 20px 0;
}

.history-list {
  margin-top: 20px;
}

.empty-history {
  padding: 40px 0;
}

.message-content {
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-list :deep(.el-table__row) {
  cursor: pointer;
}

.history-list :deep(.el-table__row:hover) {
  background-color: #f5f7fa;
}
</style>
