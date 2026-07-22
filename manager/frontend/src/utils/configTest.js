import api from './api';

/** Phân tích một mục dữ liệu từ API thành kết quả thống nhất (bao gồm first_packet_ms) */
function normItem(item) {
  if (!item || typeof item !== 'object')
    return { ok: false, message: '', first_packet_ms: undefined, reasoning_content_returned: false };
  const ms = item.first_packet_ms;
  return {
    ok: !!item.ok,
    message: item.message || '',
    first_packet_ms: typeof ms === 'number' ? ms : ms != null ? Number(ms) : undefined,
    reasoning_content_returned: !!item.reasoning_content_returned,
  };
}

/**
 * Kiểm tra một cấu hình đơn lẻ hoặc một loại cấu hình
 * @param {string} type - Loại: ota | vad | asr | llm | tts
 * @param {string} [configId] - Tùy chọn, nếu chỉ định config_id thì chỉ kiểm tra mục đó
 * @returns {Promise<{ ok: boolean, message: string, first_packet_ms?: number }>} Nếu chỉ có một mục thì trả về trực tiếp kết quả; nếu có nhiều mục thì trả về mục đầu tiên hoặc kết quả tổng hợp
 */
export async function testSingleConfig(type, configId) {
  const body = {
    types: [type],
    config_ids: configId ? { [type]: [configId] } : {},
  };
  const res = await api.post('/admin/configs/test', body, { timeout: 30000 });
  const data = res.data?.data ?? res.data;
  const typeResult = data?.[type];
  if (!typeResult || typeof typeResult !== 'object') {
    return { ok: false, message: 'Không trả về kết quả kiểm tra' };
  }
  const entries = Object.entries(typeResult).filter(([k]) => !k.startsWith('_'));
  if (configId && typeResult[configId]) {
    return normItem(typeResult[configId]);
  }
  if (entries.length === 0) {
    const err = typeResult._error || typeResult._no_client || typeResult._none;
    const msg = err && typeof err === 'object' ? (err.message || '').trim() : '';
    const fallback = typeResult._none ? 'Chưa cấu hình hoặc chưa kích hoạt' : 'Không có kết quả kiểm tra';
    return { ok: false, message: msg || fallback };
  }
  return normItem(entries[0][1]);
}

/**
 * Kiểm tra toàn bộ cấu hình của một loại, trả về kết quả theo từng config_id (dùng cho chức năng "kiểm tra tất cả" và hiển thị trên từng dòng)
 * @param {string} type - Loại: vad | asr | llm | tts
 * @returns {Promise<Record<string, { ok: boolean, message: string, first_packet_ms?: number }>>} config_id -> { ok, message, first_packet_ms? }
 */
export async function testAllConfigs(type) {
  const body = { types: [type] };
  const res = await api.post('/admin/configs/test', body, { timeout: 60000 });
  const data = res.data?.data ?? res.data;
  const typeResult = data?.[type];
  const out = {};
  if (!typeResult || typeof typeResult !== 'object') {
    return out;
  }
  const err = typeResult._error || typeResult._no_client || typeResult._none;
  const errMsg = err && typeof err === 'object' ? (err.message || '').trim() : 'Không trả về kết quả kiểm tra';
  for (const [k, v] of Object.entries(typeResult)) {
    if (k.startsWith('_')) continue;
    out[k] = normItem(v);
  }
  if (Object.keys(out).length === 0 && errMsg) {
    out._global = { ok: false, message: errMsg };
  }
  return out;
}

/**
 * Chuyển giá trị trả về của getJsonData() thành đối tượng có thể gộp (form trả về là chuỗi JSON)
 * @param {string|object} jsonData - Giá trị trả về của getJsonData()
 * @returns {object}
 */
export function parseJsonData(jsonData) {
  if (jsonData == null) return {};
  if (typeof jsonData === 'object') return jsonData;
  if (typeof jsonData !== 'string') return {};
  try {
    return JSON.parse(jsonData) || {};
  } catch {
    return {};
  }
}

/**
 * Kiểm tra bằng dữ liệu tùy chỉnh (bản nháp chưa lưu / bước hiện tại của trình hướng dẫn)
 * @param {string} type - Loại: ota | vad | asr | llm | tts
 * @param {Record<string, object>} typeData - config_id -> đối tượng cấu hình của loại này, tương ứng với data[type] của API
 * @returns {Promise<{ ok: boolean, message: string, first_packet_ms?: number }>} Kết quả của một mục duy nhất (chỉ hỗ trợ một mục)
 */
export async function testWithData(type, typeData) {
  const body = { types: [type], data: { [type]: typeData } };
  const res = await api.post('/admin/configs/test', body, { timeout: 30000 });
  const data = res.data?.data ?? res.data;
  const typeResult = data?.[type];
  if (!typeResult || typeof typeResult !== 'object') {
    return { ok: false, message: 'Không trả về kết quả kiểm tra' };
  }
  const err = typeResult._error || typeResult._no_client;
  if (err && typeof err === 'object' && err.message) {
    return { ok: false, message: err.message };
  }
  const entries = Object.entries(typeResult).filter(([k]) => !k.startsWith('_'));
  if (entries.length === 0) {
    return { ok: false, message: typeResult._none?.message || 'Không có kết quả kiểm tra' };
  }
  return normItem(entries[0][1]);
}
