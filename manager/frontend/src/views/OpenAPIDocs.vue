<template>
  <div class="vp-docs">
    <aside class="vp-sidebar">
      <div class="vp-sidebar-title">Tài liệu OpenAPI</div>
      <a v-for="item in nav" :key="item.id" :href="`#${item.id}`" class="vp-nav-item">{{ item.label }}</a>
    </aside>

    <main class="vp-content">
      <header class="vp-hero">
        <h1>Tài liệu OpenAPI Milestones</h1>
        <p class="lead">
          Truy cập công khai, cung cấp phương thức gọi, tham số đầu vào, đầu ra và ví dụ theo từng endpoint.
        </p>
        <div class="hero-meta">
          <span>Base URL: <code>/api/open/v1</code></span>
          <span>Content-Type: <code>application/json</code></span>
          <el-button size="small" type="primary" plain @click="$router.push('/login')">Quay lại đăng nhập</el-button>
        </div>
      </header>

      <section id="auth" class="vp-section">
        <h2>Phương thức xác thực</h2>
        <pre><code>Authorization: Bearer &lt;jwt-or-api-token&gt;
X-API-Token: &lt;api-token&gt;</code></pre>
      </section>

      <section id="common" class="vp-section">
        <h2>Mô tả phản hồi chung</h2>
        <ul>
          <li>
            Mã lỗi thường gặp: <code>400</code> Tham số không hợp lệ, <code>401</code> Xác thực thất bại,
            <code>404</code> Tài nguyên không tồn tại, <code>500</code> Lỗi phía máy chủ.
          </li>
          <li>Mặc định phân trang: <code>page=1</code>, <code>page_size=50</code>.</li>
        </ul>
      </section>

      <section id="profile" class="vp-section">
        <h2>1. Lấy thông tin người dùng hiện tại</h2>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/profile</code></div>
        <h4>Tham số đầu vào</h4>
        <p>Không có (chỉ cần header xác thực).</p>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{
  "user": {"id": 1, "username": "demo", "email": "demo@example.com", "role": "user"}
}</code></pre>
      </section>

      <section id="devices" class="vp-section">
        <h2>2. API Thiết bị</h2>

        <h3>2.1 Lấy danh sách thiết bị</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/devices</code></div>
        <h4>Tham số đầu vào</h4>
        <p>Không có (chỉ cần header xác thực).</p>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":[{"id":1,"device_name":"bedroom","device_code":"123456","agent_id":2,"activated":true}]}</code></pre>

        <h3>2.2 Tạo thiết bị</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/devices</code></div>
        <h4>Tham số Body</h4>
        <table>
          <thead>
            <tr>
              <th>Trường</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>device_name</td>
              <td>string</td>
              <td>Có</td>
              <td>Tên thiết bị, 2-50 ký tự</td>
            </tr>
            <tr>
              <td>agent_id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID trợ lý AI liên kết</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"success":true,"message":"Tạo thiết bị thành công","data":{"device_code":"654321","device":{"id":8,"device_name":"bedroom"}}}</code></pre>
      </section>

      <section id="agents" class="vp-section">
        <h2>3. API Trợ lý AI</h2>

        <h3>3.1 Lấy danh sách trợ lý AI</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/agents</code></div>
        <h4>Tham số đầu vào</h4>
        <p>Không có (chỉ cần header xác thực).</p>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":[{"id":2,"name":"Trợ lý gia đình","nickname":"Milestones","llm_config_id":"llm_default"}]}</code></pre>

        <h3>3.2 Tạo trợ lý AI</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/agents</code></div>
        <h4>Tham số Body</h4>
        <table>
          <thead>
            <tr>
              <th>Trường</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>name</td>
              <td>string</td>
              <td>Có</td>
              <td>Tên, 2-50 ký tự</td>
            </tr>
            <tr>
              <td>nickname</td>
              <td>string</td>
              <td>Không</td>
              <td>Tên gọi, dùng cho mô hình lớn/Prompt; để trống sẽ mặc định bằng name</td>
            </tr>
            <tr>
              <td>custom_prompt</td>
              <td>string</td>
              <td>Không</td>
              <td>Prompt tùy chỉnh</td>
            </tr>
            <tr>
              <td>llm_config_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID cấu hình LLM</td>
            </tr>
            <tr>
              <td>tts_config_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID cấu hình TTS</td>
            </tr>
            <tr>
              <td>voice</td>
              <td>string</td>
              <td>Không</td>
              <td>Định danh giọng nói</td>
            </tr>
            <tr>
              <td>asr_speed</td>
              <td>string</td>
              <td>Không</td>
              <td>Mặc định là normal</td>
            </tr>
            <tr>
              <td>memory_mode</td>
              <td>string</td>
              <td>Không</td>
              <td>short/long/none</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"success":true,"data":{"id":3,"name":"Trợ lý phòng khách","nickname":"Milestones"}}</code></pre>

        <h3>3.3 Lấy chi tiết trợ lý AI</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/agents/:id</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID trợ lý AI</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":{"id":2,"name":"Trợ lý gia đình","nickname":"Milestones","custom_prompt":"..."}}</code></pre>

        <h3>3.4 Cập nhật trợ lý AI</h3>
        <div class="api-line"><span class="method put">PUT</span><code>/api/open/v1/agents/:id</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID trợ lý AI</td>
            </tr>
          </tbody>
        </table>
        <h4>Tham số Body</h4>
        <table>
          <thead>
            <tr>
              <th>Trường</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>name</td>
              <td>string</td>
              <td>Có</td>
              <td>Tên, 2-50 ký tự</td>
            </tr>
            <tr>
              <td>nickname</td>
              <td>string</td>
              <td>Không</td>
              <td>Tên gọi, dùng cho mô hình lớn/Prompt; để trống sẽ mặc định bằng name</td>
            </tr>
            <tr>
              <td>custom_prompt</td>
              <td>string</td>
              <td>Không</td>
              <td>Prompt tùy chỉnh</td>
            </tr>
            <tr>
              <td>llm_config_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID cấu hình LLM (có thể để trống)</td>
            </tr>
            <tr>
              <td>tts_config_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID cấu hình TTS (có thể để trống)</td>
            </tr>
            <tr>
              <td>voice</td>
              <td>string</td>
              <td>Không</td>
              <td>Định danh giọng nói</td>
            </tr>
            <tr>
              <td>asr_speed</td>
              <td>string</td>
              <td>Không</td>
              <td>Để trống thì mặc định là normal</td>
            </tr>
            <tr>
              <td>memory_mode</td>
              <td>string</td>
              <td>Không</td>
              <td>short/long/none</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":{"id":2,"name":"Trợ lý gia đình - đã cập nhật","nickname":"Milestones"}}</code></pre>

        <h3>3.5 Xóa trợ lý AI</h3>
        <div class="api-line"><span class="method delete">DELETE</span><code>/api/open/v1/agents/:id</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID trợ lý AI</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"message":"Xóa thành công"}</code></pre>
      </section>

      <section id="history" class="vp-section">
        <h2>4. API Lịch sử trò chuyện</h2>

        <h3>4.1 Truy vấn tin nhắn (phân trang)</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/history/messages</code></div>
        <h4>Tham số Query</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>agent_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID trợ lý AI</td>
            </tr>
            <tr>
              <td>device_id</td>
              <td>string</td>
              <td>Không</td>
              <td>Định danh thiết bị (device_name)</td>
            </tr>
            <tr>
              <td>session_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID phiên hội thoại</td>
            </tr>
            <tr>
              <td>role</td>
              <td>string</td>
              <td>Không</td>
              <td>user/assistant</td>
            </tr>
            <tr>
              <td>page</td>
              <td>number</td>
              <td>Không</td>
              <td>Mặc định 1</td>
            </tr>
            <tr>
              <td>page_size</td>
              <td>number</td>
              <td>Không</td>
              <td>Mặc định 50</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"total":120,"page":1,"page_size":50,"data":[{"id":1,"role":"user","content":"Xin chào"}]}</code></pre>

        <h3>4.2 Xuất tin nhắn</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/history/export</code></div>
        <h4>Tham số Query</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>agent_id</td>
              <td>string</td>
              <td>Không</td>
              <td>ID trợ lý AI</td>
            </tr>
            <tr>
              <td>device_id</td>
              <td>string</td>
              <td>Không</td>
              <td>Định danh thiết bị (device_name)</td>
            </tr>
            <tr>
              <td>start_date</td>
              <td>string</td>
              <td>Không</td>
              <td>YYYY-MM-DD</td>
            </tr>
            <tr>
              <td>end_date</td>
              <td>string</td>
              <td>Không</td>
              <td>YYYY-MM-DD</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"export_time":"2026-03-17 10:00:00","total":20,"messages":[...]}</code></pre>
      </section>

      <section id="inject" class="vp-section">
        <h2>5. API Phát giọng nói</h2>
        <div class="api-line">
          <span class="method post">POST</span><code>/api/open/v1/devices/inject-message</code>
        </div>
        <h4>Tham số Body</h4>
        <table>
          <thead>
            <tr>
              <th>Trường</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>device_id</td>
              <td>string</td>
              <td>Có</td>
              <td>Định danh thiết bị (device_name)</td>
            </tr>
            <tr>
              <td>message</td>
              <td>string</td>
              <td>Có</td>
              <td>Nội dung phát</td>
            </tr>
            <tr>
              <td>skip_llm</td>
              <td>boolean</td>
              <td>Không</td>
              <td>Có bỏ qua LLM không, mặc định false</td>
            </tr>
            <tr>
              <td>auto_listen</td>
              <td>boolean</td>
              <td>Không</td>
              <td>Sau khi phát xong có tự động chuyển sang lắng nghe không, mặc định true</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"success":true,"message":"Yêu cầu phát giọng nói đã được gửi","data":{"device_id":"bedroom","message":"hello","skip_llm":false,"auto_listen":true}}</code></pre>
      </section>

      <section id="mcp" class="vp-section">
        <h2>6. API Công cụ MCP</h2>

        <h3>6.1 Lấy danh sách công cụ của trợ lý AI</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/agents/:id/mcp-tools</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID trợ lý AI</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":{"tools":[{"name":"tool_a","description":"..."}]}}</code></pre>

        <h3>6.2 Gọi công cụ của trợ lý AI</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/agents/:id/mcp-call</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID trợ lý AI</td>
            </tr>
          </tbody>
        </table>
        <h4>Tham số Body</h4>
        <table>
          <thead>
            <tr>
              <th>Trường</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>tool_name</td>
              <td>string</td>
              <td>Có</td>
              <td>Tên công cụ</td>
            </tr>
            <tr>
              <td>arguments</td>
              <td>object</td>
              <td>Không</td>
              <td>Đối tượng tham số công cụ</td>
            </tr>
          </tbody>
        </table>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":{"result":"ok"}}</code></pre>

        <h3>6.3 Lấy danh sách công cụ của thiết bị</h3>
        <div class="api-line"><span class="method get">GET</span><code>/api/open/v1/devices/:id/mcp-tools</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID thiết bị</td>
            </tr>
          </tbody>
        </table>
        <h4>Lưu ý</h4>
        <ul>
          <li>Chỉ trả về các công cụ IoT over MCP phía thiết bị tương ứng với transport đang online hiện tại.</li>
          <li>
            Không bao gồm công cụ lịch sử từ các transport khác, cũng không bao gồm công cụ ws-endpoint của trợ lý AI.
          </li>
        </ul>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":{"tools":[{"name":"device_tool","description":"..."}]}}</code></pre>

        <h3>6.4 Gọi công cụ của thiết bị</h3>
        <div class="api-line"><span class="method post">POST</span><code>/api/open/v1/devices/:id/mcp-call</code></div>
        <h4>Tham số Path</h4>
        <table>
          <thead>
            <tr>
              <th>Tham số</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>id</td>
              <td>number</td>
              <td>Có</td>
              <td>ID thiết bị</td>
            </tr>
          </tbody>
        </table>
        <h4>Tham số Body</h4>
        <table>
          <thead>
            <tr>
              <th>Trường</th>
              <th>Kiểu</th>
              <th>Bắt buộc</th>
              <th>Mô tả</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>tool_name</td>
              <td>string</td>
              <td>Có</td>
              <td>Tên công cụ</td>
            </tr>
            <tr>
              <td>arguments</td>
              <td>object</td>
              <td>Không</td>
              <td>Đối tượng tham số công cụ</td>
            </tr>
          </tbody>
        </table>
        <h4>Lưu ý</h4>
        <ul>
          <li>Ưu tiên khớp và gọi theo danh sách công cụ thiết bị hiện tại.</li>
          <li>
            Khi công cụ chưa xuất hiện trong danh sách nhưng runtime hiện tại vẫn khả dụng, máy chủ sẽ tự động thử gọi
            raw call dự phòng.
          </li>
        </ul>
        <h4>Ví dụ đầu ra</h4>
        <pre><code>{"data":{"device_id":"bedroom","tool_name":"device_tool","result":"ok"}}</code></pre>
      </section>
    </main>
  </div>
</template>

<script setup>
const nav = [
  { id: 'auth', label: 'Xác thực' },
  { id: 'common', label: 'Mô tả chung' },
  { id: 'profile', label: '1. Thông tin người dùng' },
  { id: 'devices', label: '2. API Thiết bị' },
  { id: 'agents', label: '3. API Trợ lý AI' },
  { id: 'history', label: '4. Lịch sử trò chuyện' },
  { id: 'inject', label: '5. Phát giọng nói' },
  { id: 'mcp', label: '6. Công cụ MCP' },
];
</script>

<style scoped>
.vp-docs {
  display: flex;
  gap: 24px;
  max-width: 1280px;
  margin: 0 auto;
  padding: 24px 16px 40px;
  color: #213547;
}
.vp-sidebar {
  position: sticky;
  top: 20px;
  height: calc(100vh - 40px);
  min-width: 220px;
  border-right: 1px solid #e5e7eb;
  padding-right: 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.vp-sidebar-title {
  font-weight: 700;
  margin-bottom: 8px;
}
.vp-nav-item {
  color: #4b5563;
  text-decoration: none;
  font-size: 14px;
}
.vp-nav-item:hover {
  color: #3b82f6;
}
.vp-content {
  flex: 1;
  min-width: 0;
}
.vp-hero h1 {
  margin: 0;
  font-size: 32px;
}
.lead {
  margin: 10px 0;
  color: #4b5563;
}
.hero-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.vp-section {
  margin-top: 26px;
  padding-top: 6px;
  border-top: 1px solid #f0f2f5;
}
.vp-section h2 {
  margin: 0 0 10px;
}
.vp-section h3 {
  margin: 20px 0 8px;
}
.vp-section h4 {
  margin: 14px 0 6px;
  font-size: 14px;
  color: #374151;
}
.api-line {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0;
}
.method {
  color: #fff;
  font-size: 12px;
  border-radius: 6px;
  padding: 2px 8px;
  font-weight: 600;
}
.method.get {
  background: #10b981;
}
.method.post {
  background: #3b82f6;
}
.method.put {
  background: #f59e0b;
}
.method.delete {
  background: #ef4444;
}
pre {
  margin: 6px 0;
  background: #f6f8fb;
  color: #111827;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  padding: 12px;
  overflow: auto;
  font-size: 12px;
}
code {
  background: #f3f4f6;
  padding: 2px 6px;
  border-radius: 8px;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
th,
td {
  border: 1px solid #e5e7eb;
  padding: 8px;
  text-align: left;
}
thead {
  background: #f8fafc;
}
@media (max-width: 960px) {
  .vp-docs {
    display: block;
  }
  .vp-sidebar {
    position: static;
    height: auto;
    min-width: auto;
    border-right: none;
    border-bottom: 1px solid #e5e7eb;
    padding-bottom: 12px;
    margin-bottom: 16px;
  }
}
</style>
