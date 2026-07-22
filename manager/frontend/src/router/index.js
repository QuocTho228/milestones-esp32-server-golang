import { createRouter, createWebHistory } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { isMobile } from '../utils/device';

// Tải component động theo loại thiết bị
const getLoginComponent = () => {
  return isMobile() ? import('../views/mobile/MobileLogin.vue') : import('../views/Login.vue');
};

const routes = [
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('../views/Setup.vue'),
  },
  {
    path: '/test',
    name: 'Test',
    component: () => import('../views/Test.vue'),
  },
  {
    path: '/test-route',
    name: 'TestRoute',
    component: () => import('../views/TestRoute.vue'),
  },
  {
    path: '/simple-login',
    name: 'SimpleLogin',
    component: () => import('../views/SimpleLogin.vue'),
  },
  {
    path: '/login',
    name: 'Login',
    component: getLoginComponent,
  },

  {
    path: '/openapi-docs',
    name: 'OpenAPIDocs',
    component: () => import('../views/OpenAPIDocs.vue'),
    meta: { title: 'Hướng dẫn OpenAPI' },
  },
  {
    path: '/',
    name: 'Layout',
    component: () => import('../components/Layout.vue'),
    redirect: '/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: '/dashboard',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: 'Trang Dashboard', requiresAdmin: true },
      },
      // Định tuyến quản trị viên
      {
        path: '/admin',
        name: 'Admin',
        meta: { requiresAuth: true, requiresAdmin: true },
        children: [
          {
            path: 'config-wizard',
            name: 'ConfigWizard',
            component: () => import('../views/admin/ConfigWizard.vue'),
            meta: { title: 'Trình hướng dẫn cấu hình' },
          },
          {
            path: 'vad-config',
            name: 'VADConfig',
            component: () => import('../views/admin/VADConfig.vue'),
            meta: { title: 'Quản lý cấu hình VAD' },
          },
          {
            path: 'asr-config',
            name: 'ASRConfig',
            component: () => import('../views/admin/ASRConfig.vue'),
            meta: { title: 'Quản lý cấu hình ASR' },
          },
          {
            path: 'llm-config',
            name: 'LLMConfig',
            component: () => import('../views/admin/LLMConfig.vue'),
            meta: { title: 'Quản lý cấu hình LLM' },
          },
          {
            path: 'tts-config',
            name: 'TTSConfig',
            component: () => import('../views/admin/TTSConfig.vue'),
            meta: { title: 'Quản lý cấu hình TTS' },
          },
          {
            path: 'speaker-config',
            name: 'SpeakerConfig',
            component: () => import('../views/admin/SpeakerConfig.vue'),
            meta: { title: 'Quản lý cấu hình nhận dạng giọng nói' },
          },
          {
            path: 'ota-config',
            name: 'OTAConfig',
            component: () => import('../views/admin/OTAConfig.vue'),
            meta: { title: 'Quản lý cấu hình OTA' },
          },
          {
            path: 'mqtt-config',
            name: 'MQTTConfig',
            component: () => import('../views/admin/MQTTConfig.vue'),
            meta: { title: 'Quản lý cấu hình MQTT' },
          },
          {
            path: 'udp-config',
            name: 'UDPConfig',
            component: () => import('../views/admin/UDPConfig.vue'),
            meta: { title: 'Quản lý cấu hình UDP' },
          },
          {
            path: 'mqtt-server-config',
            name: 'MQTTServerConfig',
            component: () => import('../views/admin/MQTTServerConfig.vue'),
            meta: { title: 'Quản lý cấu hình MQTT Server' },
          },
          {
            path: 'mcp-config',
            name: 'MCPConfig',
            component: () => import('../views/admin/MCPConfig.vue'),
            meta: { title: 'Quản lý cấu hình MCP' },
          },
          {
            path: 'mcp-market',
            name: 'MCPMarket',
            component: () => import('../views/admin/MCPMarket.vue'),
            meta: { title: 'Cửa hàng MCP' },
          },
          {
            path: 'memory-config',
            name: 'MemoryConfig',
            component: () => import('../views/admin/MemoryConfig.vue'),
            meta: { title: 'Quản lý cấu hình Memory' },
          },
          {
            path: 'knowledge-search-config',
            name: 'KnowledgeSearchConfig',
            component: () => import('../views/admin/KnowledgeSearchConfig.vue'),
            meta: { title: 'Cấu hình kho tri thức' },
          },
          {
            path: 'chat-settings',
            name: 'ChatSettings',
            component: () => import('../views/admin/ChatSettings.vue'),
            meta: { title: 'Cài đặt trò chuyện' },
          },
          {
            path: 'vision-config',
            name: 'VisionConfig',
            component: () => import('../views/admin/VisionConfig.vue'),
            meta: { title: 'Quản lý cấu hình Vision' },
          },
          {
            path: 'pool-stats',
            name: 'PoolStats',
            component: () => import('../views/admin/PoolStats.vue'),
            meta: { title: 'Thống kê nhóm tài nguyên' },
          },
          {
            path: 'global-roles',
            name: 'GlobalRoles',
            component: () => import('../views/admin/GlobalRoles.vue'),
            meta: { title: 'Quản lý vai trò toàn cục' },
          },
          {
            path: 'users',
            name: 'Users',
            component: () => import('../views/admin/Users.vue'),
            meta: { title: 'Quản lý người dùng' },
          },
          {
            path: 'devices',
            name: 'AdminDevices',
            component: () => import('../views/admin/Devices.vue'),
            meta: { title: 'Quản lý thiết bị' },
          },
          {
            path: 'agents',
            name: 'AdminAgents',
            component: () => import('../views/admin/Agents.vue'),
            meta: { title: 'Quản lý tác nhân AI' },
          },
        ],
      },
      // Định tuyến người dùng
      {
        path: '/console',
        redirect: '/agents',
        meta: { title: 'Công cụ làm việc của tác nhân AI' },
      },
      {
        path: '/agents',
        name: 'Agents',
        component: () => import('../views/user/Agents.vue'),
        meta: { title: 'Tác nhân AI của tôi' },
      },
      {
        path: '/user/agents',
        name: 'UserAgents',
        component: () => import('../views/user/Agents.vue'),
        meta: { title: 'Tác nhân AI của tôi' },
      },
      {
        path: '/agents/:id/edit',
        name: 'AgentEdit',
        component: () => import('../views/user/AgentEdit.vue'),
        meta: { title: 'Chỉnh sửa tác nhân AI' },
      },
      {
        path: '/user/agents/:id/edit',
        name: 'UserAgentEdit',
        component: () => import('../views/user/AgentEdit.vue'),
        meta: { title: 'Chỉnh sửa tác nhân AI' },
      },
      {
        path: '/user/agents/:id/devices',
        name: 'AgentDevices',
        component: () => import('../views/user/AgentDevices.vue'),
        meta: { title: 'Quản lý thiết bị tác nhân AI' },
      },
      {
        path: '/user/devices',
        name: 'UserDevices',
        component: () => import('../views/user/AgentDevices.vue'),
        meta: { title: 'Danh sách thiết bị' },
      },
      {
        path: '/speakers',
        name: 'Speakers',
        component: () => import('../views/user/Speakers.vue'),
        meta: { title: 'Quản lý âm thanh' },
      },
      {
        path: '/user/speakers',
        name: 'UserSpeakers',
        component: () => import('../views/user/Speakers.vue'),
        meta: { title: 'Quản lý giọng nói' },
      },
      {
        path: '/voice-clones',
        name: 'VoiceClones',
        component: () => import('../views/user/VoiceClones.vue'),
        meta: { title: 'Quản lý âm thanh' },
      },
      {
        path: '/more',
        name: 'MobileMore',
        component: () => import('../views/mobile/MobileMore.vue'),
        meta: { title: 'Nhiều tính năng hơn' },
      },
      {
        path: '/user/agents/:id/history',
        name: 'AgentHistory',
        component: () => import('../views/user/AgentHistory.vue'),
        meta: { title: 'Lịch sử trò chuyện' },
      },

      {
        path: '/user/api-tokens',
        name: 'UserAPITokens',
        component: () => import('../views/user/APITokens.vue'),
        meta: { title: 'Quản lý API Token' },
      },
      {
        path: '/user/knowledge-bases',
        name: 'UserKnowledgeBases',
        component: () => import('../views/user/KnowledgeBases.vue'),
        meta: { title: 'Kho tri thức của tôi' },
      },
      {
        path: 'user/roles',
        name: 'UserRoles',
        component: () => import('../views/user/Roles.vue'),
        meta: { title: 'Vai trò của tôi' },
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore();

  // Nếu truy cập trang khởi động, hãy đi thẳng qua
  if (to.path === '/setup') {
    next();
    return;
  }

  // Nếu truy cập trang đăng nhập và đã đăng nhập, hãy nhảy theo vai trò (quản trị viên không hoàn thành trình hướng dẫn lần đầu tiên, sau đó chuyển đến trình hướng dẫn cấu hình)
  if (to.path === '/login' && authStore.isAuthenticated) {
    if (authStore.user?.role === 'admin') {
      if (!localStorage.getItem('admin_first_login_done')) {
        next('/admin/config-wizard');
      } else {
        next('/dashboard');
      }
    } else {
      next('/agents');
    }
    return;
  }

  // Nếu cần xác thực
  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      // Không có token, chuyển đến trang đăng nhập
      next('/login');
      return;
    }

    // Có token nhưng chưa có thông tin người dùng, thử xác thực tính hợp lệ của token
    if (!authStore.user && !authStore.isValidating) {
      try {
        await authStore.getProfile();
      } catch (error) {
        // Nếu là lỗi 401 (token không hợp lệ), chuyển đến trang đăng nhập
        if (error.response?.status === 401) {
          next('/login');
          return;
        }
        // Nếu là lỗi mạng (kết nối backend thất bại), cho phép tiếp tục truy cập (nhưng sẽ hiển thị lỗi)
        if (
          error.code === 'ERR_NETWORK' ||
          error.message?.includes('Failed to fetch') ||
          error.message?.includes('ERR_CONNECTION_REFUSED')
        ) {
          // Khi có lỗi mạng, nếu đã có thông tin người dùng cục bộ thì cho phép tiếp tục truy cập
          if (!authStore.user) {
            next('/login');
            return;
          }
          // Lưu ý: ở đây không gọi next(), để code tiếp tục chạy đến next() cuối cùng
        } else {
          // Lỗi khác, cho phép tiếp tục truy cập (có thể backend tạm thời không khả dụng)
          // Lưu ý: ở đây không gọi next(), để code tiếp tục chạy đến next() cuối cùng
        }
      }
    }

    // Nếu đang trong quá trình xác thực, đợi xác thực hoàn tất (tối đa 2 giây)
    if (authStore.isValidating) {
      let waitCount = 0;
      while (authStore.isValidating && waitCount < 20) {
        await new Promise((resolve) => setTimeout(resolve, 100));
        waitCount++;
      }
    }
  }

  // Nếu truy cập đường dẫn gốc, chuyển hướng theo vai trò (quản trị viên chưa hoàn thành hướng dẫn lần đầu thì đến trình hướng dẫn cấu hình)
  if (to.path === '/' && authStore.isAuthenticated) {
    if (authStore.user?.role === 'admin') {
      if (!localStorage.getItem('admin_first_login_done')) {
        next('/admin/config-wizard');
      } else {
        next('/dashboard');
      }
    } else {
      next('/agents');
    }
    return;
  }

  // Nếu người dùng thường truy cập trang quản trị, chuyển đến không gian làm việc tác nhân AI
  if (to.meta.requiresAdmin && authStore.user?.role !== 'admin') {
    next('/agents');
    return;
  }

  next();
});

export default router;
