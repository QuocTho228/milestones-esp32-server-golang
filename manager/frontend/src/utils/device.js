/**
 * Công cụ nhận diện thiết bị
 * Dùng để xác định loại thiết bị đang truy cập, phục vụ bố cục responsive
 */

/**
 * Kiểm tra có phải thiết bị di động hay không
 * @returns {boolean}
 */
export const isMobile = () => {
  // Xác định thông qua User-Agent
  const userAgent = navigator.userAgent || navigator.vendor || window.opera;
  const mobileRegex = /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i;
  const isMobileUA = mobileRegex.test(userAgent);

  // Xác định thông qua chiều rộng màn hình (phương án dự phòng)
  const isMobileWidth = window.innerWidth < 768;

  return isMobileUA || isMobileWidth;
};

/**
 * Kiểm tra có phải máy tính bảng hay không
 * @returns {boolean}
 */
export const isTablet = () => {
  const userAgent = navigator.userAgent || navigator.vendor || window.opera;
  return /iPad|Android/i.test(userAgent) && window.innerWidth >= 768 && window.innerWidth < 1024;
};

/**
 * Kiểm tra có phải thiết bị desktop hay không
 * @returns {boolean}
 */
export const isDesktop = () => {
  return !isMobile() && !isTablet();
};

/**
 * Kiểm tra có phải trình duyệt WeChat hay không
 * @returns {boolean}
 */
export const isWeChat = () => {
  const userAgent = navigator.userAgent || '';
  return /MicroMessenger/i.test(userAgent);
};

/**
 * Lấy loại thiết bị
 * @returns {'mobile' | 'tablet' | 'desktop'}
 */
export const getDeviceType = () => {
  if (isMobile()) {
    return 'mobile';
  } else if (isTablet()) {
    return 'tablet';
  } else {
    return 'desktop';
  }
};

/**
 * Lắng nghe sự thay đổi kích thước cửa sổ
 * @param {Function} callback Hàm callback
 * @returns {Function} Hàm hủy lắng nghe
 */
export const onResize = (callback) => {
  let ticking = false;

  const handler = () => {
    if (!ticking) {
      window.requestAnimationFrame(() => {
        callback(getDeviceType());
        ticking = false;
      });
      ticking = true;
    }
  };

  window.addEventListener('resize', handler);

  // Trả về hàm để hủy lắng nghe
  return () => {
    window.removeEventListener('resize', handler);
  };
};
