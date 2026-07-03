// Script chẩn đoán frontend
console.log('=== Bắt đầu chẩn đoán frontend ===');

// Kiểm tra môi trường cơ bản
console.log('1. Kiểm tra môi trường cơ bản:');
console.log('   - Phiên bản Vue:', typeof window.Vue !== 'undefined' ? 'Vue đã được tải' : 'Vue chưa được tải');
console.log('   - URL hiện tại:', window.location.href);
console.log('   - User Agent:', navigator.userAgent);

// Kiểm tra localStorage
console.log('2. Kiểm tra lưu trữ cục bộ:');
console.log('   - Token:', localStorage.getItem('token'));
console.log('   - User:', localStorage.getItem('user'));

// Kiểm tra kết nối mạng
console.log('3. Kiểm tra kết nối backend:');
fetch('http://localhost:8080/api/profile')
  .then((response) => {
    console.log('   - Trạng thái phản hồi từ backend:', response.status);
    if (response.status === 401) {
      console.log('   - Backend hoạt động bình thường (trả về lỗi 401 chưa xác thực)');
    }
  })
  .catch((error) => {
    console.log('   - Kết nối backend thất bại:', error.message);
  });

// Kiểm tra route
console.log('4. Các route kiểm tra khả dụng:');
console.log('   - /test - Trang kiểm tra cơ bản');
console.log('   - /simple-login - Trang đăng nhập đơn giản');
console.log('   - /login - Trang đăng nhập đầy đủ');

console.log('=== Kết thúc chẩn đoán frontend ===');
console.log('Vui lòng xem thông tin trên trong console của trình duyệt');

// Xuất ra global, tiện gọi trong console
window.diagnose = () => {
  console.clear();
  // Chạy lại chẩn đoán
  location.reload();
};
