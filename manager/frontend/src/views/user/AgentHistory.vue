<template>
  <div class="agent-history-page">
    <div class="page-header">
      <div class="header-left">
        <el-button @click="$router.back()" :icon="ArrowLeft" circle size="large" />
        <div class="header-context">
          <span class="context-label">Trợ lý AI hiện tại</span>
          <strong class="context-value">{{ agentName || 'Trợ lý AI chưa đặt tên' }}</strong>
          <p class="context-meta" v-if="total > 0">Tổng cộng {{ total }} tin nhắn</p>
        </div>
      </div>
      <div class="header-right">
        <el-button @click="handleExport" :loading="exporting">
          <el-icon><Download /></el-icon>
          Xuất lịch sử
        </el-button>
      </div>
    </div>

    <!-- Bảng lọc -->
    <el-card class="filter-card" shadow="never">
      <el-form :model="filters" inline>
        <el-form-item label="Vai trò">
          <el-select v-model="filters.role" placeholder="Tất cả" clearable style="width: 120px">
            <el-option label="Tất cả" value="" />
            <el-option label="Người dùng" value="user" />
            <el-option label="Trợ lý AI" value="assistant" />
          </el-select>
        </el-form-item>
        <el-form-item label="Thiết bị">
          <el-select v-model="filters.device_id" placeholder="Tất cả" clearable style="width: 150px">
            <el-option label="Tất cả" value="" />
            <el-option
              v-for="device in devices"
              :key="device.id"
              :label="device.device_name || device.device_code"
              :value="device.device_name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Ngày bắt đầu">
          <el-date-picker
            v-model="filters.start_date"
            type="date"
            placeholder="Chọn ngày"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            style="width: 150px"
            clearable
          />
        </el-form-item>
        <el-form-item label="Ngày kết thúc">
          <el-date-picker
            v-model="filters.end_date"
            type="date"
            placeholder="Chọn ngày"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            style="width: 150px"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">Tìm kiếm</el-button>
          <el-button @click="handleReset">Đặt lại</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Danh sách tin nhắn - kiểu chat -->
    <el-card class="messages-card" shadow="never" v-loading="loading">
      <div v-if="messages.length === 0" class="empty-state">
        <el-empty description="Chưa có lịch sử trò chuyện" />
      </div>
      <div v-else class="chat-container">
        <div class="chat-messages" ref="chatMessagesRef">
          <div
            v-for="(message, index) in messages"
            :key="message.id"
            class="message-wrapper"
            :class="{ 'message-right': message.role === 'user', 'message-left': message.role === 'assistant' }"
          >
            <!-- Dấu thời gian (hiển thị nếu cách tin nhắn trước hơn 5 phút) -->
            <div v-if="shouldShowTime(message, index)" class="message-time-divider">
              {{ formatTimeShort(message.created_at) }}
            </div>

            <div class="message-bubble-wrapper">
              <!-- Bên trái: tin nhắn trợ lý AI -->
              <template v-if="message.role === 'assistant'">
                <div class="message-bubble message-bubble-left">
                  <div class="message-content-wrapper">
                    <!-- Nội dung văn bản -->
                    <div v-if="message.content" class="message-text">{{ message.content }}</div>
                    <!-- Trình phát âm thanh -->
                    <div v-if="message.audio_path" class="audio-bubble">
                      <audio
                        :ref="(el) => (audioRefs[message.id] = el)"
                        :src="audioBlobUrls[message.id]"
                        @ended="handleAudioEnded(message.id)"
                        @error="handleAudioError(message.id)"
                      />
                      <el-button
                        :icon="playingAudioId === message.id ? VideoPause : VideoPlay"
                        circle
                        size="small"
                        @click="toggleAudio(message.id)"
                        class="audio-play-btn-simple"
                      />
                    </div>
                    <div class="message-meta">
                      <span class="message-time-small">{{ formatTimeShort(message.created_at) }}</span>
                      <el-dropdown trigger="click" @command="handleMessageAction">
                        <el-icon class="message-more"><MoreFilled /></el-icon>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item :command="{ action: 'delete', id: message.id }">Xóa</el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                    </div>
                  </div>
                </div>
              </template>

              <!-- Bên phải: tin nhắn người dùng -->
              <template v-else>
                <div class="message-bubble message-bubble-right">
                  <div class="message-content-wrapper">
                    <!-- Nội dung văn bản -->
                    <div v-if="message.content" class="message-text">{{ message.content }}</div>
                    <!-- Trình phát âm thanh -->
                    <div v-if="message.audio_path" class="audio-bubble">
                      <audio
                        :ref="(el) => (audioRefs[message.id] = el)"
                        :src="audioBlobUrls[message.id]"
                        @ended="handleAudioEnded(message.id)"
                        @error="handleAudioError(message.id)"
                      />
                      <el-button
                        :icon="playingAudioId === message.id ? VideoPause : VideoPlay"
                        circle
                        size="small"
                        @click="toggleAudio(message.id)"
                        class="audio-play-btn-simple"
                      />
                    </div>
                    <div class="message-meta">
                      <el-dropdown trigger="click" @command="handleMessageAction">
                        <el-icon class="message-more"><MoreFilled /></el-icon>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item :command="{ action: 'delete', id: message.id }">Xóa</el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                      <span class="message-time-small">{{ formatTimeShort(message.created_at) }}</span>
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>

        <!-- Phân trang -->
        <div class="pagination" v-if="total > 0">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :total="total"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="handlePageChange"
          />
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, computed, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { ArrowLeft, Download, User, Service, VideoPlay, VideoPause, MoreFilled } from '@element-plus/icons-vue';
import api from '../../utils/api';

const route = useRoute();
const router = useRouter();

const agentId = computed(() => {
  const id = route.params.id;
  return id ? String(id) : null;
});
const agentName = ref('');
const loading = ref(false);
const exporting = ref(false);
const messages = ref([]);
const total = ref(0);
const devices = ref([]);
const deletingId = ref(null);

// Điều kiện lọc
const filters = reactive({
  role: '',
  device_id: '',
  start_date: '',
  end_date: '',
});

// Phân trang
const pagination = reactive({
  page: 1,
  pageSize: 50,
});

// Tính tổng số trang
const totalPages = computed(() => {
  return Math.ceil(total.value / pagination.pageSize);
});

// Liên quan phát âm thanh
const audioRefs = ref({});
const playingAudioId = ref(null);
const chatMessagesRef = ref(null);
const audioBlobUrls = ref({}); // Lưu trữ Blob URL âm thanh

// Tải thông tin trợ lý AI
const loadAgent = async () => {
  if (!agentId.value) {
    ElMessage.error('ID trợ lý AI không hợp lệ');
    router.back();
    return;
  }
  try {
    const response = await api.get(`/user/agents/${agentId.value}`);
    agentName.value = response.data.data?.name || 'Trợ lý AI';
  } catch (error) {
    console.error('Tải thông tin trợ lý AI thất bại:', error);
    ElMessage.error('Tải thông tin trợ lý AI thất bại');
  }
};

// Tải danh sách thiết bị
const loadDevices = async () => {
  try {
    const response = await api.get(`/user/agents/${agentId.value}/devices`);
    devices.value = response.data.data || [];
  } catch (error) {
    console.error('TảiThiết bịdanh sách thất bại:', error);
  }
};

// Tải danh sách tin nhắn
const loadMessages = async () => {
  if (!agentId.value) {
    return;
  }
  loading.value = true;
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
    };
    if (filters.role) params.role = filters.role;
    if (filters.device_id) params.device_id = filters.device_id;
    if (filters.start_date) params.start_date = filters.start_date;
    if (filters.end_date) params.end_date = filters.end_date;

    const response = await api.get(`/user/history/agents/${agentId.value}/messages`, { params });
    // Backend trả về theo thứ tự thời gian giảm dần (mới nhất trước), cần đảo mảng để mới nhất ở dưới
    const data = response.data.data || [];
    messages.value = [...data].reverse(); // Đảo mảng, mới nhất ở dưới
    total.value = response.data.total || 0;

    // Tải trước tin nhắn có âm thanh
    await preloadAudioMessages();

    // Sau khi tải xongCuộn xuống dưới（hiển thị tin nhắn mới nhất）
    await nextTick();
    scrollToBottom();
  } catch (error) {
    ElMessage.error('Tải danh sách tin nhắn thất bại: ' + (error.response?.data?.error || error.message));
    console.error('Tải danh sách tin nhắn thất bại:', error);
    messages.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
};

// Tìm kiếm
const handleSearch = () => {
  pagination.page = 1;
  loadMessages();
};

// Đặt lại bộ lọc
const handleReset = () => {
  filters.role = '';
  filters.device_id = '';
  filters.start_date = '';
  filters.end_date = '';
  pagination.page = 1;
  loadMessages();
};

// Thay đổi phân trang
const handlePageChange = (page) => {
  pagination.page = page;
  loadMessages();
};

const handleSizeChange = (size) => {
  pagination.pageSize = size;
  pagination.page = 1;
  loadMessages();
};

// Xóa tin nhắn
const handleDelete = async (messageId) => {
  try {
    await ElMessageBox.confirm('Bạn có chắc muốn xóa tin nhắn này không?', 'Thông báo', {
      confirmButtonText: 'Xác nhận',
      cancelButtonText: 'Hủy',
      type: 'warning',
    });

    deletingId.value = messageId;
    await api.delete(`/user/history/messages/${messageId}`);
    ElMessage.success('Xóa thành công');
    loadMessages();
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Xóa thất bại');
      console.error('Xóa tin nhắn thất bại:', error);
    }
  } finally {
    deletingId.value = null;
  }
};

// Xuất lịch sử
const handleExport = async () => {
  exporting.value = true;
  try {
    const params = {
      agent_id: agentId.value,
    };
    if (filters.role) params.role = filters.role;
    if (filters.device_id) params.device_id = filters.device_id;
    if (filters.start_date) params.start_date = filters.start_date;
    if (filters.end_date) params.end_date = filters.end_date;

    const response = await api.get('/user/history/export', {
      params,
      responseType: 'blob',
    });

    // Tạo liên kết tải xuống
    const url = window.URL.createObjectURL(new Blob([response.data]));
    const link = document.createElement('a');
    link.href = url;
    link.setAttribute('download', `chat_history_${new Date().toISOString().slice(0, 10)}.json`);
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.URL.revokeObjectURL(url);

    ElMessage.success('Xuất thành công');
  } catch (error) {
    ElMessage.error('Xuất thất bại');
    console.error('Xuất thất bại:', error);
  } finally {
    exporting.value = false;
  }
};

// Định dạng thời gian (đầy đủ)
const formatTime = (dateString) => {
  const date = new Date(dateString);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
};

// Định dạng thời gian (ngắn, dùng cho bong bóng tin nhắn)
const formatTimeShort = (dateString) => {
  const date = new Date(dateString);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const msgDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  // Nếu là hôm nay, chỉ hiển thị giờ
  if (msgDate.getTime() === today.getTime()) {
    return date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  // Nếu là hôm qua
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);
  if (msgDate.getTime() === yesterday.getTime()) {
    return (
      'Hôm qua ' +
      date.toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
      })
    );
  }

  // Nếu là năm nay, hiển thị ngày tháng và giờ
  if (date.getFullYear() === now.getFullYear()) {
    return `${date.getMonth() + 1}tháng ${date.getDate()} ngày ${date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    })}`;
  }

  // Trường hợp khác hiển thị ngày giờ đầy đủ
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
};

// Kiểm tra có hiển thị dấu phân cách thời gian không
const shouldShowTime = (message, index) => {
  if (index === 0) return true;
  const currentTime = new Date(message.created_at).getTime();
  const prevTime = new Date(messages.value[index - 1].created_at).getTime();
  // Nếu cách tin nhắn trước hơn 5 phút, hiển thị thời gian
  return currentTime - prevTime > 5 * 60 * 1000;
};

// Xử lý thao tác tin nhắn
const handleMessageAction = (command) => {
  if (command.action === 'delete') {
    handleDelete(command.id);
  }
};

// Cuộn xuống dưới
const scrollToBottom = () => {
  if (chatMessagesRef.value) {
    nextTick(() => {
      chatMessagesRef.value.scrollTop = chatMessagesRef.value.scrollHeight;
    });
  }
};

// Lấy URL âm thanh (dùng Blob URL để hỗ trợ xác thực)
const getAudioUrl = async (messageId) => {
  // Nếu đã có Blob URL, trả về trực tiếp
  if (audioBlobUrls.value[messageId]) {
    return audioBlobUrls.value[messageId];
  }

  try {
    // Dùng axios lấy dữ liệu âm thanh (tự động đính kèm token xác thực)
    const response = await api.get(`/user/history/messages/${messageId}/audio`, {
      responseType: 'blob', // Quan trọng: chỉ định kiểu phản hồi là blob
    });

    // Tạo Blob URL
    const blobUrl = URL.createObjectURL(response.data);
    audioBlobUrls.value[messageId] = blobUrl;

    return blobUrl;
  } catch (error) {
    // Xử lý im lặng, chỉ ghi log, không hiển thị lỗi
    console.warn('Tải âm thanh thất bại:', messageId, error);
    return null;
  }
};

// Tải trước tin nhắn âm thanh
const preloadAudioMessages = async () => {
  const audioMessages = messages.value.filter((msg) => msg.audio_path);
  // Tải trước đồng thời nhưng giới hạn số lượng
  const promises = audioMessages.slice(0, 10).map((msg) =>
    getAudioUrl(msg.id).catch((err) => {
      console.warn('Tải trước âm thanh thất bại:', msg.id, err);
      return null;
    }),
  );
  await Promise.all(promises);
};

// Âm thanh phát xong
const handleAudioEnded = (messageId) => {
  playingAudioId.value = null;
};

// Xử lý lỗi tải âm thanh
const handleAudioError = async (messageId) => {
  // Xử lý im lặng, chỉ ghi log, không hiển thị lỗi
  console.warn('Tải âm thanh thất bại:', messageId);
  // Thử tải lại
  try {
    const url = await getAudioUrl(messageId);
    if (url) {
      const audio = audioRefs.value[messageId];
      if (audio) {
        audio.load(); // Tải lại âm thanh
      }
    }
  } catch (error) {
    // Xử lý im lặng, chỉ ghi log
    console.warn('Tải lại âm thanh thất bại:', messageId, error);
  }
};

// Chuyển đổi phát âm thanh
const toggleAudio = async (messageId) => {
  const audio = audioRefs.value[messageId];
  if (!audio) return;

  // Nếu chưa tải âm thanh, tải trước
  if (!audioBlobUrls.value[messageId]) {
    const url = await getAudioUrl(messageId);
    if (!url) {
      // Xử lý im lặng, chỉ ghi log, không hiển thị lỗi
      console.warn('Tải âm thanh thất bại, không thể phát:', messageId);
      return;
    }
    // Chờ phần tử âm thanh tải
    await new Promise((resolve) => {
      audio.onloadeddata = resolve;
      audio.load();
    });
  }

  // Dừng âm thanh khác
  if (playingAudioId.value && playingAudioId.value !== messageId) {
    const otherAudio = audioRefs.value[playingAudioId.value];
    if (otherAudio) {
      otherAudio.pause();
      otherAudio.currentTime = 0;
    }
  }

  if (playingAudioId.value === messageId) {
    // Tạm dừng âm thanh hiện tại
    audio.pause();
    playingAudioId.value = null;
  } else {
    // Phát âm thanh
    try {
      await audio.play();
      playingAudioId.value = messageId;
    } catch (error) {
      // Xử lý im lặng, chỉ ghi log, không hiển thị lỗi
      console.warn('Phát âm thanh thất bại:', messageId, error);
    }
  }
};

onMounted(async () => {
  if (!agentId.value) {
    ElMessage.error('ID trợ lý AI không hợp lệ');
    router.push('/user/agents');
    return;
  }
  try {
    await Promise.all([loadAgent(), loadDevices(), loadMessages()]);
  } catch (error) {
    console.error('Khởi tạo thất bại:', error);
  }
});

// Dọn dẹp Blob URL khi tháo component để tránh rò rỉ bộ nhớ
onBeforeUnmount(() => {
  Object.values(audioBlobUrls.value).forEach((url) => {
    if (url) {
      URL.revokeObjectURL(url);
    }
  });
  audioBlobUrls.value = {};
});
</script>

<style scoped>
.agent-history-page {
  padding: 0;
  background: transparent;
  min-height: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 20px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(255, 255, 255, 0.9);
  border-radius: var(--apple-radius-lg);
  box-shadow: var(--apple-shadow-md);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-context {
  display: grid;
  gap: 4px;
}

.context-label {
  color: var(--apple-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.context-value {
  color: var(--apple-text);
  font-size: 16px;
  line-height: 1.3;
}

.context-meta {
  margin: 0;
  color: var(--apple-text-secondary);
  font-size: 14px;
}

.filter-card {
  margin-bottom: 20px;
}

.messages-card {
  min-height: 400px;
}

.empty-state {
  padding: 60px 0;
  text-align: center;
}

.chat-container {
  background: rgba(248, 250, 252, 0.92);
  border: 1px solid rgba(229, 229, 234, 0.72);
  min-height: 500px;
  border-radius: 22px;
  overflow: hidden;
}

.chat-messages {
  padding: 20px;
  max-height: 70vh;
  overflow-y: auto;
}

.message-wrapper {
  display: flex;
  flex-direction: column;
  margin-bottom: 16px;
}

.message-time-divider {
  text-align: center;
  margin: 16px 0;
  font-size: 12px;
  color: var(--apple-text-tertiary);
}

.message-bubble-wrapper {
  display: flex;
  align-items: flex-start;
  max-width: 75%;
}

.message-right {
  margin-left: auto;
  justify-content: flex-end;
  width: 100%;
  display: flex;
}

.message-left {
  margin-right: auto;
  justify-content: flex-start;
  width: 100%;
  display: flex;
}

/* Bong bóng tin nhắn */
.message-bubble {
  position: relative;
  padding: 10px 14px;
  border-radius: 18px;
  word-wrap: break-word;
  word-break: break-word;
  box-shadow: 0 8px 16px rgba(15, 23, 42, 0.05);
  max-width: 100%;
}

.message-bubble-left {
  background: rgba(255, 255, 255, 0.94);
  border-top-left-radius: 8px;
}

.message-bubble-right {
  background: rgba(0, 122, 255, 0.12);
  border: 1px solid rgba(0, 122, 255, 0.16);
  border-top-right-radius: 8px;
  margin-left: auto;
}

.message-content-wrapper {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.message-text {
  color: var(--apple-text);
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 14px;
}

.message-bubble-right .message-text {
  color: var(--apple-text);
}

/* Bong bóng âm thanh */
.audio-bubble {
  margin: 4px 0;
  display: flex;
  align-items: center;
}

.audio-play-btn-simple {
  flex-shrink: 0;
}

/* Thông tin meta tin nhắn */
.message-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  opacity: 0.7;
}

.message-meta:hover {
  opacity: 1;
}

.message-time-small {
  font-size: 11px;
  color: var(--apple-text-tertiary);
}

.message-bubble-right .message-time-small {
  color: var(--apple-primary-pressed);
}

.message-more {
  font-size: 14px;
  color: var(--apple-text-tertiary);
  cursor: pointer;
  padding: 2px;
  border-radius: 8px;
  transition: all 0.2s;
}

.message-more:hover {
  background: rgba(0, 122, 255, 0.08);
  color: var(--apple-primary);
}

.message-bubble-right .message-more {
  color: var(--apple-primary-pressed);
}

.message-bubble-right .message-more:hover {
  background: rgba(0, 122, 255, 0.12);
}

/* Phân trang */
.pagination {
  margin-top: 20px;
  padding: 20px;
  display: flex;
  justify-content: center;
  background: rgba(255, 255, 255, 0.88);
  border-top: 1px solid rgba(229, 229, 234, 0.72);
}

/* Kiểu thanh cuộn */
.chat-messages::-webkit-scrollbar {
  width: 6px;
}

.chat-messages::-webkit-scrollbar-track {
  background: rgba(229, 229, 234, 0.52);
  border-radius: 3px;
}

.chat-messages::-webkit-scrollbar-thumb {
  background: rgba(142, 142, 147, 0.58);
  border-radius: 3px;
}

.chat-messages::-webkit-scrollbar-thumb:hover {
  background: rgba(110, 110, 115, 0.68);
}

/* Ghi đè kiểu Element Plus */
:deep(.el-slider__runway) {
  margin: 0;
  height: 4px;
}

:deep(.el-slider__bar) {
  height: 4px;
}

:deep(.el-slider__button) {
  width: 12px;
  height: 12px;
  border: 2px solid var(--apple-primary);
}

:deep(.el-slider__button-wrapper) {
  width: 24px;
  height: 24px;
  top: -10px;
}

:deep(.el-dropdown-menu__item) {
  padding: 8px 20px;
}
</style>
