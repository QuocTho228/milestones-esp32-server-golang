const defaultOption = { label: 'Mặc định', value: 'default' };
const enableOption = { label: 'Bật', value: 'enabled' };
const disableOption = { label: 'Tắt', value: 'disabled' };
const clearHistoryOptions = [
  { label: 'Mặc định', value: 'default' },
  { label: 'Xoá', value: true },
  { label: 'Giữ nguyên', value: false },
];

function withDefault(options) {
  return [defaultOption, ...options];
}

function createModel(value, thinking, extra = {}) {
  return {
    value,
    label: value,
    thinking,
    ...extra,
  };
}

const openAIReasoningStandard = withDefault([
  { label: 'Rất thấp', value: 'minimal' },
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
]);

const openAIReasoningCodex = withDefault([
  { label: 'Tắt', value: 'none' },
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
]);

const openAIReasoningCodexMax = withDefault([
  { label: 'Tắt', value: 'none' },
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
  { label: 'Rất cao', value: 'xhigh' },
]);

const openAIReasoningLegacy = withDefault([
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
]);

const openAIReasoningHighOnly = withDefault([{ label: 'Cao', value: 'high' }]);

const booleanThinkingOptions = withDefault([enableOption, disableOption]);

const doubaoReasoningOptions = withDefault([
  { label: 'Tắt', value: 'minimal' },
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
]);

const anthropicAdaptiveOptions = [
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
  { label: 'Rất cao', value: 'max' },
];

const openAIReasoningLatest = withDefault([
  { label: 'Tắt', value: 'none' },
  { label: 'Thấp', value: 'low' },
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
  { label: 'Rất cao', value: 'xhigh' },
]);

const openAIReasoningLatestPro = withDefault([
  { label: 'Trung bình', value: 'medium' },
  { label: 'Cao', value: 'high' },
  { label: 'Rất cao', value: 'xhigh' },
]);

const openAIReasoningRequest = {
  allowMaxTokens: false,
  allowTemperature: false,
  allowTopP: false,
};

const anthropicManualThinking = {
  label: 'Suy nghĩ sâu',
  options: withDefault([{ label: 'Tư duy thủ công', value: 'enabled' }]),
  showBudgetFor: ['enabled'],
  budgetMin: 1024,
  budgetRequiredFor: ['enabled'],
};

const anthropicAdaptiveThinking = {
  label: 'Suy nghĩ sâu',
  options: withDefault([
    { label: 'Tư duy thủ công', value: 'enabled' },
    { label: 'Tư duy thích nghi', value: 'adaptive' },
  ]),
  showBudgetFor: ['enabled'],
  budgetMin: 1024,
  budgetRequiredFor: ['enabled'],
  showEffortFor: ['adaptive'],
  effortOptions: anthropicAdaptiveOptions,
};

const zhipuThinkingConfig = {
  label: 'Suy nghĩ sâu',
  options: booleanThinkingOptions,
  showClearThinkingFor: ['enabled'],
  clearThinkingOptions: clearHistoryOptions,
};

const aliyunThinkingConfig = {
  label: 'Suy nghĩ sâu',
  options: booleanThinkingOptions,
  showBudgetFor: ['enabled'],
  budgetMin: 1,
  budgetStep: 256,
};

const siliconflowThinkingConfig = {
  label: 'Suy nghĩ sâu',
  options: booleanThinkingOptions,
  showBudgetFor: ['enabled'],
  budgetMin: 128,
  budgetMax: 32768,
  budgetStep: 128,
};

const providerTypeMap = {
  openai: 'openai',
  ollama: 'ollama',
  azure: 'openai',
  anthropic: 'openai',
  zhipu: 'openai',
  aliyun: 'openai',
  doubao: 'openai',
  siliconflow: 'openai',
  deepseek: 'openai',
  dify: 'dify',
  coze: 'coze',
};

const knownProviders = new Set(Object.keys(providerTypeMap));

const editableBaseURLProviders = new Set(['openai', 'ollama', 'azure', 'dify', 'coze']);

const catalog = {
  openai: {
    quickUrl: 'https://api.openai.com/v1',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint:
      'Mặc định ưu tiên dùng tên tắt ổn định của nhà phát hành; nếu cần cố định hành vi, hãy nhập Model ID snapshot chính xác.',
    models: [
      createModel(
        'gpt-5.4',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatest },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.4-pro',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatestPro },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.4-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatest },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.4-nano',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatest },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.2',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatest },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.2-pro',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatestPro },
        { request: openAIReasoningRequest },
      ),
      createModel('gpt-5-chat-latest', false, {
        hint: 'Tên tắt dành riêng cho ChatGPT, phù hợp với các luồng làm việc cũ; khi tích hợp mới nên ưu tiên chọn mô hình GPT-5.* chính.',
      }),
      createModel(
        'gpt-5-pro',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningHighOnly },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningStandard },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningStandard },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-nano',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningStandard },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.3-codex',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodexMax },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.2-codex',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodexMax },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-codex',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.1',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodex },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.1-codex',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodex },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.1-codex-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodex },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.1-codex-max',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodexMax },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o3',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o4-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o3-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o1',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
    ],
    fallbackThinking: {
      label: 'Mức độ suy nghĩ',
      options: openAIReasoningCodex,
      hint: 'Mô hình tuỳ chỉnh không có trong danh sách tài liệu, đã chuyển sang cấu hình reasoning_effort chung; hiệu lực phụ thuộc vào mô hình thực tế.',
    },
  },
  ollama: {
    quickUrl: 'http://127.0.0.1:11434/v1',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint:
      'Ollama sử dụng dịch vụ mô hình cục bộ hoặc riêng tư — danh sách mô hình và địa chỉ đều có thể tuỳ chỉnh.',
    models: [],
    fallbackThinking: null,
  },
  azure: {
    quickUrl: 'https://your-resource-name.openai.azure.com/openai/v1/',
    modelPlaceholder: 'Chọn tên mô hình chính thức hoặc nhập tên deployment tuỳ chỉnh',
    modelHint: 'Trường này là Deployment Name trên Azure; danh sách tên chỉ để tham khảo năng lực mô hình nền.',
    models: [
      createModel(
        'gpt-5.4',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatest },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.4-pro',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatestPro },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.2',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLatest },
        { request: openAIReasoningRequest },
      ),
      createModel('gpt-5.2-chat', false, {
        hint: 'Các model Chat trong tài liệu Azure thường được truy cập qua tên deployment; khả năng mở phụ thuộc vào khu vực và hạn ngạch.',
      }),
      createModel(
        'gpt-5.3-codex',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodexMax },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5.2-codex',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningCodexMax },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningStandard },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-nano',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningStandard },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-chat',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningStandard },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'gpt-5-pro',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningHighOnly },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o4-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o3',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o3-mini',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
      createModel(
        'o1',
        { label: 'Mức độ suy nghĩ', options: openAIReasoningLegacy },
        { request: openAIReasoningRequest },
      ),
    ],
    fallbackThinking: {
      label: 'Mức độ suy nghĩ',
      options: openAIReasoningCodex,
      hint: 'Khi Azure deployment tuỳ chỉnh không có trong danh sách tài liệu, sẽ dùng cấu hình reasoning_effort chung; tính tương thích phụ thuộc vào mô hình đã triển khai.',
    },
  },
  anthropic: {
    quickUrl: 'https://api.anthropic.com/v1/',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint:
      'Mặc định ưu tiên dùng tên tắt ổn định; nếu cần cố định phiên bản hoặc kiểm thử hồi quy, hãy nhập Model ID có ngày tháng chính xác.',
    models: [
      createModel('claude-opus-4-6', anthropicAdaptiveThinking),
      createModel('claude-sonnet-4-6', anthropicAdaptiveThinking),
      createModel('claude-haiku-4-5', anthropicManualThinking),
      createModel('claude-3-7-sonnet', anthropicManualThinking),
      createModel('claude-sonnet-4', anthropicManualThinking),
      createModel('claude-opus-4', anthropicManualThinking),
      createModel('claude-opus-4-1', anthropicManualThinking),
    ],
    fallbackThinking: {
      ...anthropicAdaptiveThinking,
      hint: 'Mô hình tuỳ chỉnh không có trong tài liệu. Khi dùng tư duy thủ công, phải nhập budget_tokens rõ ràng; chế độ Adaptive chỉ dùng cho mô hình được tài liệu xác nhận hỗ trợ.',
    },
  },
  zhipu: {
    quickUrl: 'https://open.bigmodel.cn/api/paas/v4',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint: 'Tài liệu Zhipu hỗ trợ điều khiển chế độ suy nghĩ qua thinking.type và clear_thinking.',
    models: [
      createModel('glm-5', zhipuThinkingConfig),
      createModel('glm-4.7', zhipuThinkingConfig),
      createModel('glm-4.7-flashx', zhipuThinkingConfig),
      createModel('glm-4.7-flash', zhipuThinkingConfig),
      createModel('glm-4.6', zhipuThinkingConfig),
      createModel('glm-4.6v', zhipuThinkingConfig),
      createModel('glm-4.5', zhipuThinkingConfig),
      createModel('glm-4.5-air', zhipuThinkingConfig),
      createModel('glm-4.5-airx', zhipuThinkingConfig),
      createModel('glm-4.5v', zhipuThinkingConfig),
    ],
    fallbackThinking: {
      ...zhipuThinkingConfig,
      hint: 'Mô hình tuỳ chỉnh không có trong tài liệu, đã chuyển sang cấu hình thinking.type / clear_thinking chung.',
    },
  },
  aliyun: {
    quickUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint:
      'Mặc định ưu tiên dùng tên tắt ổn định; nếu muốn cố định phiên bản cụ thể, hãy nhập Model ID kèm ngày hoặc hậu tố phiên bản nhỏ.',
    models: [
      createModel('qwen-plus-latest', aliyunThinkingConfig),
      createModel('qwen-turbo-latest', aliyunThinkingConfig),
      createModel('qwen3-max', aliyunThinkingConfig),
      createModel('qwen3-235b-a22b', aliyunThinkingConfig),
      createModel('qwen3-30b-a3b', aliyunThinkingConfig),
      createModel('qwen3-next-80b-a3b-thinking', aliyunThinkingConfig),
      createModel('glm-4.7', aliyunThinkingConfig),
      createModel('glm-4.6', aliyunThinkingConfig),
      createModel('glm-4.5', aliyunThinkingConfig),
      createModel('glm-4.5-air', aliyunThinkingConfig),
      createModel('kimi-k2-thinking', aliyunThinkingConfig),
      createModel('qwen3-235b-a22b-thinking-2507', aliyunThinkingConfig, {
        label: 'qwen3-235b-a22b-thinking-2507（Phiên bản cố định）',
      }),
      createModel('qwen3-30b-a3b-thinking-2507', aliyunThinkingConfig, {
        label: 'qwen3-30b-a3b-thinking-2507（Phiên bản cố định）',
      }),
      createModel('kimi/kimi-k2.5', aliyunThinkingConfig, { label: 'kimi/kimi-k2.5（Phiên bản cố định）' }),
    ],
    fallbackThinking: {
      ...aliyunThinkingConfig,
      hint: 'Mô hình tuỳ chỉnh không có trong tài liệu. Nếu mô hình hỗ trợ thinking_budget, điền theo tài liệu; để trống sẽ không truyền trường này.',
    },
  },
  doubao: {
    quickUrl: 'https://ark.cn-beijing.volces.com/api/v3',
    modelPlaceholder: 'Chọn hoặc nhập Model ID (thường kèm hậu tố phiên bản)',
    modelHint:
      'Doubao ưu tiên nhập Model ID chính thức. Hiện chưa xác nhận có tên tắt ổn định, nên dùng Model ID từ bảng điều khiển hoặc danh sách mô hình.',
    models: [
      createModel(
        'doubao-seed-2-0-pro-260215',
        { label: 'Mức độ suy nghĩ', options: doubaoReasoningOptions },
        { label: 'Doubao Seed 2.0 Pro (doubao-seed-2-0-pro-260215)' },
      ),
      createModel(
        'doubao-seed-2-0-lite-260215',
        { label: 'Mức độ suy nghĩ', options: doubaoReasoningOptions },
        { label: 'Doubao Seed 2.0 Lite (doubao-seed-2-0-lite-260215)' },
      ),
      createModel(
        'doubao-seed-2-0-mini-260215',
        { label: 'Mức độ suy nghĩ', options: doubaoReasoningOptions },
        { label: 'Doubao Seed 2.0 Mini (doubao-seed-2-0-mini-260215)' },
      ),
      createModel(
        'doubao-seed-1-6-251015',
        { label: 'Mức độ suy nghĩ', options: doubaoReasoningOptions },
        { label: 'Doubao Seed 1.6 (doubao-seed-1-6-251015)' },
      ),
    ],
    fallbackThinking: {
      label: 'Mức độ suy nghĩ',
      options: doubaoReasoningOptions,
      hint: 'Mô hình tuỳ chỉnh không có trong danh sách tài liệu, đã chuyển sang cấu hình reasoning_effort chung; hiệu lực phụ thuộc vào mô hình thực tế.',
    },
  },
  siliconflow: {
    quickUrl: 'https://api.siliconflow.cn/v1',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint:
      'Tài liệu SiliconFlow liệt kê trực tiếp các mô hình hỗ trợ enable_thinking; cấu hình ngân sách chỉ hiển thị cho các mô hình trong danh sách.',
    models: [
      createModel('Pro/zai-org/GLM-5', siliconflowThinkingConfig),
      createModel('Pro/zai-org/GLM-4.7', siliconflowThinkingConfig),
      createModel('deepseek-ai/DeepSeek-V3.2', siliconflowThinkingConfig),
      createModel('Pro/deepseek-ai/DeepSeek-V3.2', siliconflowThinkingConfig),
      createModel('zai-org/GLM-4.6', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-8B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-14B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-32B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3-30B-A3B', siliconflowThinkingConfig),
      createModel('tencent/Hunyuan-A13B-Instruct', siliconflowThinkingConfig),
      createModel('zai-org/GLM-4.5V', siliconflowThinkingConfig),
      createModel('deepseek-ai/DeepSeek-V3.1-Terminus', siliconflowThinkingConfig),
      createModel('Pro/deepseek-ai/DeepSeek-V3.1-Terminus', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-397B-A17B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-122B-A10B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-35B-A3B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-27B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-9B', siliconflowThinkingConfig),
      createModel('Qwen/Qwen3.5-4B', siliconflowThinkingConfig),
    ],
    fallbackThinking: {
      ...siliconflowThinkingConfig,
      hint: 'Mô hình tuỳ chỉnh không có trong tài liệu. Nếu mô hình hỗ trợ enable_thinking / thinking_budget, điền theo tài liệu; để trống sẽ không truyền thinking_budget.',
    },
  },
  deepseek: {
    quickUrl: 'https://api.deepseek.com/v1',
    modelPlaceholder: 'Vui lòng chọn hoặc nhập tên mô hình',
    modelHint:
      'DeepSeek chuyển chế độ suy nghĩ bằng cách chọn mô hình: deepseek-chat (không suy nghĩ), deepseek-reasoner (có suy nghĩ).',
    models: [
      createModel('deepseek-chat', false, {
        hint: 'deepseek-chat là mô hình không có suy nghĩ sâu, không cần tham số thinking.',
      }),
      createModel('deepseek-reasoner', false, {
        hint: 'deepseek-reasoner đã tích hợp sẵn chế độ suy nghĩ sâu, không cần tham số thinking riêng.',
      }),
    ],
    fallbackThinking: {
      label: 'Suy nghĩ sâu',
      options: booleanThinkingOptions,
      hint: 'DeepSeek khuyến nghị đổi chế độ suy nghĩ bằng tên mô hình. Nếu proxy tuỳ chỉnh hỗ trợ thêm thinking.type, có thể bật công tắc tương thích tại đây.',
    },
  },
};

function cloneOptions(options = []) {
  return options.map((option) => ({ ...option }));
}

function normalizeModelName(modelName) {
  return String(modelName || '')
    .trim()
    .toLowerCase();
}

export function resolveLLMProvider(provider, type) {
  const normalizedProvider = String(provider || '')
    .trim()
    .toLowerCase();
  const normalizedType = String(type || '')
    .trim()
    .toLowerCase();

  if (normalizedProvider === 'openai' && ['ollama', 'dify', 'coze'].includes(normalizedType)) {
    return normalizedType;
  }
  if (knownProviders.has(normalizedProvider)) {
    return normalizedProvider;
  }
  if (['ollama', 'dify', 'coze'].includes(normalizedType)) {
    return normalizedType;
  }
  return 'openai';
}

export function getProviderFixedType(provider) {
  return providerTypeMap[provider] || 'openai';
}

export function isProviderBaseURLEditable(provider) {
  return editableBaseURLProviders.has(provider);
}

export function getProviderQuickUrl(provider) {
  return catalog[provider]?.quickUrl || '';
}

export function getProviderModelOptions(provider) {
  return (catalog[provider]?.models || []).map((model) => ({
    label: model.label,
    value: model.value,
  }));
}

export function getProviderModelHint(provider) {
  return catalog[provider]?.modelHint || '';
}

export function getProviderModelFieldLabel(provider) {
  if (provider === 'azure') {
    return 'Tên triển khai (Deployment)';
  }
  if (provider === 'doubao') {
    return 'Model ID';
  }
  return 'Tên mô hình';
}

export function getProviderModelPlaceholder(provider) {
  return catalog[provider]?.modelPlaceholder || 'Vui lòng chọn hoặc nhập tên mô hình';
}

export function resolveProviderModel(provider, modelName) {
  const normalized = normalizeModelName(modelName);
  if (!normalized) {
    return null;
  }

  const models = catalog[provider]?.models || [];
  return models.find((model) => normalizeModelName(model.value) === normalized) || null;
}

export function getProviderRequestConfig(provider, modelName) {
  const model = resolveProviderModel(provider, modelName);
  return {
    allowMaxTokens: true,
    allowTemperature: true,
    allowTopP: true,
    temperatureMax: 2,
    ...(model?.request || {}),
  };
}

export function getProviderThinkingConfig(provider, modelName) {
  const model = resolveProviderModel(provider, modelName);
  if (model?.thinking === false) {
    return {
      visible: false,
      hint: model.hint || '',
    };
  }

  const source = model?.thinking || catalog[provider]?.fallbackThinking;
  if (!source) {
    return {
      visible: false,
      hint: model?.hint || '',
    };
  }

  return {
    visible: true,
    label: source.label || 'Suy nghĩ sâu',
    options: cloneOptions(source.options),
    showBudgetFor: [...(source.showBudgetFor || [])],
    budgetMin: source.budgetMin || 1,
    budgetMax: source.budgetMax || 100000,
    budgetStep: source.budgetStep || 1,
    budgetRequiredFor: [...(source.budgetRequiredFor || [])],
    showEffortFor: [...(source.showEffortFor || [])],
    effortOptions: cloneOptions(source.effortOptions || []),
    showClearThinkingFor: [...(source.showClearThinkingFor || [])],
    clearThinkingOptions: cloneOptions(source.clearThinkingOptions || clearHistoryOptions),
    hint: model?.hint || source.hint || '',
  };
}
