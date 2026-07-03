<template>
  <el-form ref="formRef" :model="model" :rules="rules" label-width="120px">
    <el-form-item label="Nhà cung cấp" prop="provider">
      <el-select v-model="model.provider" placeholder="Vui lòng chọn nhà cung cấp" style="width: 100%">
        <el-option label="TEN VAD" value="ten_vad" />
        <el-option label="Silero VAD" value="silero_vad" />
      </el-select>
    </el-form-item>
    <el-form-item label="Tên cấu hình" prop="name">
      <el-input v-model="model.name" placeholder="Vui lòng nhập tên cấu hình" />
    </el-form-item>
    <el-form-item label="ID cấu hình" prop="config_id">
      <el-input v-model="model.config_id" placeholder="Vui lòng nhập ID cấu hình duy nhất" />
    </el-form-item>
    <template v-if="model.provider === 'webrtc_vad'">
      <el-divider content-position="left">Cấu hình WebRTC VAD</el-divider>
      <el-form-item label="Kích thước pool tối thiểu" prop="webrtc_vad.pool_min_size">
        <el-input-number v-model="model.webrtc_vad.pool_min_size" :min="1" :max="1000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Kích thước pool tối đa" prop="webrtc_vad.pool_max_size">
        <el-input-number v-model="model.webrtc_vad.pool_max_size" :min="1" :max="10000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Số kết nối rảnh tối đa" prop="webrtc_vad.pool_max_idle">
        <el-input-number v-model="model.webrtc_vad.pool_max_idle" :min="1" :max="1000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Tần số lấy mẫu VAD" prop="webrtc_vad.vad_sample_rate">
        <el-select v-model="model.webrtc_vad.vad_sample_rate" style="width: 100%">
          <el-option label="8000 Hz" :value="8000" />
          <el-option label="16000 Hz" :value="16000" />
          <el-option label="32000 Hz" :value="32000" />
          <el-option label="48000 Hz" :value="48000" />
        </el-select>
      </el-form-item>
      <el-form-item label="Chế độ VAD" prop="webrtc_vad.vad_mode">
        <el-select v-model="model.webrtc_vad.vad_mode" style="width: 100%">
          <el-option label="Chế độ 0 (Ưu tiên chất lượng)" :value="0" />
          <el-option label="Chế độ 1 (Độ trễ thấp)" :value="1" />
          <el-option label="Chế độ 2 (Cân bằng)" :value="2" />
          <el-option label="Chế độ 3 (Độ chính xác cao)" :value="3" />
        </el-select>
      </el-form-item>
    </template>
    <template v-if="model.provider === 'silero_vad'">
      <el-divider content-position="left">Cấu hình Silero VAD</el-divider>
      <el-form-item label="Đường dẫn mô hình" prop="silero_vad.model_path">
        <el-input v-model="model.silero_vad.model_path" placeholder="Vui lòng nhập đường dẫn tệp mô hình" />
      </el-form-item>
      <el-form-item label="Ngưỡng" prop="silero_vad.threshold">
        <el-input-number
          v-model="model.silero_vad.threshold"
          :min="0"
          :max="1"
          :step="0.1"
          :precision="2"
          style="width: 100%"
        />
      </el-form-item>
      <el-form-item label="Thời gian im lặng tối thiểu (ms)" prop="silero_vad.min_silence_duration_ms">
        <el-input-number v-model="model.silero_vad.min_silence_duration_ms" :min="10" :max="5000" style="width: 100%" />
      </el-form-item>
      <el-form-item label="Tần số lấy mẫu" prop="silero_vad.sample_rate">
        <el-select v-model="model.silero_vad.sample_rate" style="width: 100%">
          <el-option label="8000 Hz" :value="8000" />
          <el-option label="16000 Hz" :value="16000" />
        </el-select>
      </el-form-item>
      <el-form-item label="Số kênh âm thanh" prop="silero_vad.channels">
        <el-select v-model="model.silero_vad.channels" style="width: 100%">
          <el-option label="Đơn kênh (Mono)" :value="1" />
          <el-option label="Hai kênh (Stereo)" :value="2" />
        </el-select>
      </el-form-item>
      <el-form-item label="Số phiên (Session)" prop="silero_vad.pool_size">
        <el-input-number v-model="model.silero_vad.pool_size" :min="1" :max="100" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px">Mặc định: số nhân CPU</div>
      </el-form-item>
      <el-form-item label="Thời gian chờ lấy tài nguyên (ms)" prop="silero_vad.acquire_timeout_ms">
        <el-input-number v-model="model.silero_vad.acquire_timeout_ms" :min="100" :max="30000" style="width: 100%" />
      </el-form-item>
    </template>
    <template v-if="model.provider === 'ten_vad'">
      <el-divider content-position="left">Cấu hình TEN VAD</el-divider>
      <el-form-item label="Kích thước bước khung (hop size)" prop="ten_vad.hop_size">
        <el-input-number v-model="model.ten_vad.hop_size" :min="128" :max="1024" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px">Mặc định: 320</div>
      </el-form-item>
      <el-form-item label="Ngưỡng phát hiện VAD" prop="ten_vad.threshold">
        <el-input-number
          v-model="model.ten_vad.threshold"
          :min="0"
          :max="1"
          :step="0.1"
          :precision="2"
          style="width: 100%"
        />
        <div style="font-size: 12px; color: #909399; margin-top: 4px">Giá trị khuyến nghị: 0.3</div>
      </el-form-item>
      <el-form-item label="Kích thước pool kết nối" prop="ten_vad.pool_size">
        <el-input-number v-model="model.ten_vad.pool_size" :min="1" :max="100" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px">Giá trị khuyến nghị: 10</div>
      </el-form-item>
      <el-form-item label="Thời gian chờ lấy tài nguyên (ms)" prop="ten_vad.acquire_timeout_ms">
        <el-input-number v-model="model.ten_vad.acquire_timeout_ms" :min="100" :max="30000" style="width: 100%" />
        <div style="font-size: 12px; color: #909399; margin-top: 4px">Giá trị khuyến nghị: 3000</div>
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { ref, computed } from 'vue';

const props = defineProps({
  model: { type: Object, required: true },
  rules: { type: Object, default: () => ({}) },
});

const formRef = ref();

function getJsonData() {
  const m = props.model;
  if (m.provider === 'webrtc_vad') return JSON.stringify(m.webrtc_vad || {});
  if (m.provider === 'silero_vad') return JSON.stringify(m.silero_vad || {});
  if (m.provider === 'ten_vad') return JSON.stringify(m.ten_vad || {});
  return '{}';
}

function validate(callback) {
  return formRef.value?.validate(callback);
}

function resetFields() {
  formRef.value?.resetFields();
}

defineExpose({ validate, getJsonData, resetFields });
</script>
