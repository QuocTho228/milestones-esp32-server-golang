<template>
  <div class="pool-stats">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>Thống kê Resource Pool</span>
          <div class="header-actions">
            <el-button type="primary" size="small" @click="refreshStats">
              <el-icon><Refresh /></el-icon>
              Làm mới
            </el-button>
            <el-select v-model="viewType" size="small" style="width: 120px; margin-left: 10px" disabled>
              <el-option label="Dữ liệu mới nhất" value="latest" />
            </el-select>
          </div>
        </div>
      </template>

      <!-- Tóm tắt thống kê -->
      <el-row :gutter="20" style="margin-bottom: 20px">
        <el-col :span="6">
          <el-statistic title="Tổng số bản ghi" :value="summary.total_records || 0" />
        </el-col>
        <el-col :span="6">
          <div class="stat-item">
            <div class="stat-title">Hình thức lưu trữ</div>
            <div class="stat-value">Chỉ lưu dữ liệu mới nhất</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="stat-item">
            <div class="stat-title">Thời điểm sớm nhất</div>
            <div class="stat-value">{{ formatTime(summary.oldest_timestamp) }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="stat-item">
            <div class="stat-title">Thời điểm mới nhất</div>
            <div class="stat-value">{{ formatTime(summary.newest_timestamp) }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- Dữ liệu thống kê mới nhất -->
      <div v-if="viewType === 'latest' && latestStats">
        <el-divider>Dữ liệu thống kê mới nhất（{{ formatTime(latestStats.timestamp) }}）</el-divider>
        <el-table :data="formatStatsData(latestStats.stats)" border stripe style="width: 100%" v-if="latestStats.stats">
          <el-table-column prop="poolKey" label="Resource Pool" width="200" />
          <el-table-column prop="total" label="Tổng tài nguyên" width="120" />
          <el-table-column prop="available" label="Tài nguyên khả dụng" width="120" />
          <el-table-column prop="inUse" label="Đang sử dụng" width="120" />
          <el-table-column prop="maxSize" label="Dung lượng tối đa" width="120" />
          <el-table-column prop="minSize" label="Dung lượng tối thiểu" width="120" />
          <el-table-column prop="maxIdle" label="Rảnh tối đa" width="120" />
          <el-table-column prop="isClosed" label="Trạng thái" width="100">
            <template #default="{ row }">
              <el-tag :type="row.isClosed ? 'danger' : 'success'">
                {{ row.isClosed ? 'Đã đóng' : 'Đang chạy' }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- Trạng thái trống -->
      <el-empty v-if="!latestStats" description="Chưa có dữ liệu thống kê" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import api from '@/utils/api';
import { ElMessage } from 'element-plus';
import { Refresh } from '@element-plus/icons-vue';

const viewType = ref('latest');
const latestStats = ref(null);
const summary = ref({
  total_records: 0,
  storage_duration: 'Chỉ lưu dữ liệu mới nhất',
  oldest_timestamp: null,
  newest_timestamp: null,
});

let refreshTimer = null;

onMounted(() => {
  loadSummary();
  loadStats();
  // Tự động làm mới mỗi 30 giây
  refreshTimer = setInterval(() => {
    loadStats();
  }, 30000);
});

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
  }
});

// Tải tóm tắt thống kê
const loadSummary = async () => {
  try {
    const response = await api.get('/admin/pool/stats/summary');
    // Backend trả về định dạng: { data: { data: {...} } }
    summary.value = response.data?.data || {};
  } catch (error) {
    console.error('Tải tóm tắt thống kê thất bại:', error);
  }
};

// Tải dữ liệu thống kê
const loadStats = async () => {
  try {
    const response = await api.get('/admin/pool/stats?type=latest');
    console.log('Phản hồi dữ liệu thống kê mới nhất:', response);
    // Backend trả về định dạng: { data: { timestamp: "...", stats: {...} } }
    // axios tự động parse, nên response.data là { data: {...} } từ backend
    // Cần lấy thêm một tầng data
    latestStats.value = response.data?.data || response.data || null;
    console.log('Dữ liệu mới nhất sau khi parse:', latestStats.value);
  } catch (error) {
    console.error('Tải dữ liệu thống kê thất bại:', error);
    ElMessage.error('Tải dữ liệu thống kê thất bại');
  }
};

// Làm mới dữ liệu thống kê
const refreshStats = () => {
  loadSummary();
  loadStats();
  ElMessage.success('Làm mới thành công');
};

// Định dạng dữ liệu thống kê
const formatStatsData = (stats) => {
  if (!stats || typeof stats !== 'object') {
    return [];
  }

  const result = [];
  for (const [poolKey, poolStats] of Object.entries(stats)) {
    if (poolStats && typeof poolStats === 'object') {
      result.push({
        poolKey,
        total: poolStats.total_resources || 0,
        available: poolStats.available_resources || 0,
        inUse: poolStats.in_use_resources || 0,
        maxSize: poolStats.max_size || 0,
        minSize: poolStats.min_size || 0,
        maxIdle: poolStats.max_idle || 0,
        isClosed: poolStats.is_closed || false,
      });
    }
  }
  return result;
};

// Định dạng thời gian
const formatTime = (timestamp) => {
  if (!timestamp) {
    return '-';
  }
  const date = new Date(timestamp);
  return date.toLocaleString('vi-VN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
};
</script>

<style scoped>
.pool-stats {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
}

.el-statistic {
  text-align: center;
}

.el-timeline {
  padding-left: 20px;
}

.stat-item {
  text-align: center;
  padding: 10px;
}

.stat-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}
</style>
