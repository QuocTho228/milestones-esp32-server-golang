<template>
  <div class="api-tokens-page">
    <div class="page-actions">
      <router-link class="doc-link" to="/openapi-docs">Xem tài liệu OpenAPI công khai</router-link>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        Tạo Token
      </el-button>
    </div>

    <el-alert type="info" :closable="false" show-icon>
      <template #title>
        Hỗ trợ hai phương thức gọi: Authorization: Bearer &lt;token&gt; hoặc X-API-Token: &lt;token&gt;
      </template>
    </el-alert>

    <el-card class="table-card" shadow="never">
      <el-table :data="tokens" v-loading="loading" empty-text="Chưa có Token, vui lòng tạo mới">
        <el-table-column prop="name" label="Tên" min-width="180" />
        <el-table-column prop="token_prefix" label="Tiền tố" min-width="140" />
        <el-table-column label="Trạng thái" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'">{{ row.is_active ? 'Khả dụng' : 'Đã thu hồi' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Sử dụng lần cuối" min-width="170">
          <template #default="{ row }">{{ formatTime(row.last_used_at) }}</template>
        </el-table-column>
        <el-table-column label="Thời hạn hết hạn" min-width="170">
          <template #default="{ row }">{{ formatTime(row.expires_at) }}</template>
        </el-table-column>
        <el-table-column label="Thời gian tạo" min-width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="Thao tác" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" :disabled="!row.is_active" @click="handleRevoke(row)"> Thu hồi </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showCreate" title="Tạo API Token" width="480px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="Token Tên" prop="name">
          <el-input v-model="form.name" maxlength="100" placeholder="Ví dụ: Gọi môi trường sản xuất" />
        </el-form-item>
        <el-form-item label="Số ngày hiệu lực">
          <el-input-number v-model="form.expires_in_days" :min="0" :max="3650" />
          <div class="form-tip">0 là không bao giờ hết hạn</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">Hủy</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">Tạo</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showPlainToken" title="Vui lòng lưu Token ngay" width="640px">
      <el-alert type="warning" :closable="false" show-icon>
        Token văn bản thuần không thể xem lại sau này, vui lòng sao chép và lưu trữ an toàn ngay.
      </el-alert>
      <el-input class="token-input" v-model="latestToken" type="textarea" :rows="3" readonly />
      <template #footer>
        <el-button @click="showPlainToken = false">Đóng</el-button>
        <el-button type="primary" @click="copyToken">Sao chép Token</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Plus } from '@element-plus/icons-vue';
import api from '../../utils/api';

const loading = ref(false);
const creating = ref(false);
const tokens = ref([]);
const showCreate = ref(false);
const showPlainToken = ref(false);
const latestToken = ref('');
const formRef = ref();

const form = reactive({
  name: '',
  expires_in_days: 0,
});

const rules = {
  name: [{ required: true, message: 'Vui lòng nhập tên Token', trigger: 'blur' }],
};

const formatTime = (val) => {
  if (!val) return '-';
  return new Date(val).toLocaleString();
};

const loadTokens = async () => {
  loading.value = true;
  try {
    const res = await api.get('/user/api-tokens');
    tokens.value = res.data.data || [];
  } finally {
    loading.value = false;
  }
};

const openCreateDialog = () => {
  form.name = '';
  form.expires_in_days = 0;
  showCreate.value = true;
};

const handleCreate = async () => {
  if (!formRef.value) return;
  await formRef.value.validate();

  creating.value = true;
  try {
    const res = await api.post('/user/api-tokens', form);
    latestToken.value = res.data?.data?.token || '';
    showCreate.value = false;
    showPlainToken.value = true;
    ElMessage.success('Tạo Token thành công');
    await loadTokens();
  } finally {
    creating.value = false;
  }
};

const handleRevoke = async (row) => {
  await ElMessageBox.confirm(`Bạn có chắc muốn thu hồi Token「${row.name}」không?`, 'Thông báo', {
    confirmButtonText: 'Xác nhận',
    cancelButtonText: 'Hủy',
    type: 'warning',
  });
  await api.delete(`/user/api-tokens/${row.id}`);
  ElMessage.success('Token Đã thu hồi');
  await loadTokens();
};

const copyToken = async () => {
  if (!latestToken.value) return;
  await navigator.clipboard.writeText(latestToken.value);
  ElMessage.success('Đã sao chép Token');
};

onMounted(loadTokens);
</script>

<style scoped>
.api-tokens-page {
  padding: 8px;
}
.page-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.doc-link {
  display: inline-block;
  color: var(--apple-primary);
  text-decoration: none;
  font-size: 13px;
}
.doc-link:hover {
  text-decoration: underline;
}
.table-card {
  margin-top: 12px;
}
.form-tip {
  color: #909399;
  font-size: 12px;
  margin-top: 6px;
}
.token-input {
  margin-top: 12px;
}
</style>
