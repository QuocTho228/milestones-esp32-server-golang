<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script>
import { onMounted } from 'vue';
import { useRouter } from 'vue-router';
import api from '@/utils/api';

export default {
  name: 'App',
  setup() {
    const router = useRouter();

    const checkSystemStatus = async () => {
      try {
        // Kiểm tra xem hệ thống có cần được khởi tạo hay không
        const response = await api.get('/setup/status');

        if (response.data.needs_setup) {
          // Nếu việc khởi tạo là bắt buộc và bạn hiện không ở trang khởi động, hãy chuyển đến trang khởi động
          if (router.currentRoute.value.path !== '/setup') {
            router.push('/setup');
          }
        }
      } catch (error) {
        console.error('检查系统状态失败:', error);
        // Nếu kiểm tra không thành công thì có thể mạng có vấn đề và bước nhảy không bị ép buộc
      }
    };

    onMounted(() => {
      checkSystemStatus();
    });
  },
};
</script>

<style>
#app {
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  height: 100dvh;
}

html,
body {
  height: 100%;
}

body {
  margin: 0;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

/* Tối ưu hóa kích thước trên điện thoại */
@media (max-width: 767px) {
  /* Tối ưu hóa kích thước chữ trên thiết bị di động */
  body {
    font-size: 14px;
    -webkit-text-size-adjust: 100%;
    -webkit-tap-highlight-color: transparent;
  }

  /* Tối ưu hóa cuộn di động */
  * {
    -webkit-overflow-scrolling: touch;
  }

  /* Tối ưu hóa độ trễ khi nhấn trên thiết bị di động */
  a,
  button,
  input,
  textarea {
    touch-action: manipulation;
  }

  /* Ẩn các phần tử trên màn hình */
  .desktop-only {
    display: none !important;
  }
}

/* Máy tính bàn */
@media (min-width: 768px) {
  /* Ẩn các phần tử trên màn hình */
  .mobile-only {
    display: none !important;
  }
}

/* Animation toàn cục */
.fade-enter-active,
.fade-leave-active {
  transition:
    opacity 0.22s ease,
    transform 0.22s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(4px);
}

/* responsive cho thiết bị di động */
@supports (padding: max(0px)) {
  .mobile-safe-top {
    padding-top: max(20px, env(safe-area-inset-top));
  }

  .mobile-safe-bottom {
    padding-bottom: max(20px, env(safe-area-inset-bottom));
  }
}
</style>
