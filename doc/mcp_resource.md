# Tài liệu Các loại nội dung trả về khi gọi công cụ MCP

## Tổng quan

Tài liệu này mô tả chi tiết các loại nội dung trả về khi gọi công cụ (tool) mà chương trình hỗ trợ. Chương trình sử dụng **hệ thống phản hồi có cấu trúc (structured response)**, hỗ trợ xử lý và hiển thị (render) nhiều loại nội dung khác nhau.

## 🔧 Quy trình xử lý cốt lõi

### Xử lý phản hồi khi gọi công cụ

Bộ xử lý cốt lõi cho phản hồi khi gọi công cụ chịu trách nhiệm:

1. **Thực thi lệnh gọi công cụ**: Duyệt qua tất cả các yêu cầu gọi công cụ
2. **Phân tích kết quả**: Phân tích (parse) kết quả trả về từ công cụ
3. **Nhận diện loại nội dung**: Xử lý khác nhau tùy theo loại nội dung
4. **Hiển thị tài nguyên**: Xử lý các loại nội dung khác nhau như audio, text, resource link...

## 📋 Các loại nội dung được hỗ trợ

### 1. Nội dung âm thanh (AudioContent)

**Loại**: `mcp_go.AudioContent`

**Đặc điểm**:

- Chứa dữ liệu âm thanh được mã hóa Base64
- Hỗ trợ nhiều định dạng âm thanh (MIME Type)
- Phát trực tiếp, dừng các bước xử lý LLM tiếp theo

**Quy trình xử lý**:

```go
if audioContent, ok := content.(mcp_go.AudioContent); ok {
    // Giải mã dữ liệu âm thanh Base64
    rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Data)
    // Sử dụng music_player để phát âm thanh
    audioChan, err := play_music.PlayMusicFromAudioData(ctx, rawAudioData, ...)
    // Gửi thông điệp trạng thái phát
    l.serverTransport.SendSentenceStart(playText)
    // Phát âm thanh thông qua TTS Manager
    l.ttsManager.SendTTSAudio(ctx, audioChan, true)
}
```

**Kịch bản sử dụng**:

- Công cụ phát nhạc
- Công cụ tổng hợp giọng nói (TTS)
- Phát file âm thanh

### 2. Liên kết tài nguyên (ResourceLink)

**Loại**: `mcp_go.ResourceLink`

**Đặc điểm**:

- Chứa URI tài nguyên và metadata
- Hỗ trợ đọc phân trang cho tài nguyên lớn
- Xử lý theo luồng (streaming), phù hợp cho file lớn
- Sử dụng cơ chế Pipe để phát âm thanh dạng luồng theo thời gian thực

**Quy trình xử lý**:

```go
if resourceLink, ok := content.(mcp_go.ResourceLink); ok {
    // Tạo Pipe dùng để truyền dữ liệu dạng luồng
    pipeReader, pipeWriter = io.Pipe()

    // Khởi chạy goroutine đọc phân trang
    go func() {
        // Đọc tài nguyên theo trang
        resourceResult, err := client.ReadResource(readCtx, mcp_go.ReadResourceRequest{
            Params: mcp_go.ReadResourceParams{
                URI: resourceLink.URI,
                Arguments: map[string]any{
                    "url": resourceLink.Description,
                    "start": start,
                    "end": start + page
                },
            },
        })

        // Xử lý BlobResourceContents
        for _, content := range resourceResult.Contents {
            if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
                // Giải mã và gửi vào channel luồng âm thanh
                rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)
                streamChan <- rawAudioData
            }
        }
    }()

    // Sử dụng music_player để phát luồng âm thanh
    audioChan, err := play_music.PlayMusicFromPipe(ctx, pipeReader, ...)
}
```

**Giải thích chi tiết tham số đọc phân trang**:

#### Định dạng tham số yêu cầu

```go
Arguments: map[string]any{
    "url": resourceLink.Description,  // URL tài nguyên thực tế
    "start": start,                   // Vị trí byte bắt đầu
    "end": start + page,              // Vị trí byte kết thúc
}
```

#### Giải thích tham số

- **url**: Địa chỉ URL của tài nguyên thực tế, lấy từ `resourceLink.Description`
- **start**: Vị trí byte bắt đầu, tính từ 0
- **end**: Vị trí byte kết thúc (không bao gồm), tức phạm vi đọc là [start, end)
- **Kích thước trang**: Được định nghĩa bởi hằng số `McpReadResourcePageSize`, mặc định 100KB

#### Quy trình đọc phân trang

```go
start := 0
page := McpReadResourcePageSize  // 100 * 1024
totalRead := 0
pageCount := 0

for {
    // Tạo context có timeout
    readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

    // Gửi yêu cầu đọc phân trang
    resourceResult, err := client.ReadResource(readCtx, mcp_go.ReadResourceRequest{
        Params: mcp_go.ReadResourceParams{
            URI: resourceLink.URI,
            Arguments: map[string]any{
                "url": resourceLink.Description,
                "start": start,
                "end": start + page
            },
        },
    })
    cancel()

    // Xử lý BlobResourceContents trả về
    for _, content := range resourceResult.Contents {
        if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
            // Giải mã dữ liệu Base64
            rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)

            // Kiểm tra có phải cờ kết thúc hay không
            if string(rawAudioData) == McpReadResourceStreamDoneFlag {
                return nil // Đọc hoàn tất
            }

            // Gửi vào channel luồng âm thanh
            streamChan <- rawAudioData
            totalRead += len(rawAudioData)
        }
    }

    // Kiểm tra điều kiện hoàn tất đọc
    if len(rawAudioData) < page || !hasData {
        return nil // Đọc hoàn tất
    }

    // Cập nhật vị trí bắt đầu
    start += page
    pageCount++
}
```

#### Cơ chế xử lý theo luồng (streaming)

**Kiến trúc truyền tải qua Pipe**:

```go
// Tạo Pipe dùng để truyền luồng âm thanh
pipeReader, pipeWriter = io.Pipe()

// Khởi chạy goroutine ghi dữ liệu
go func() {
    for {
        select {
        case audioData, ok := <-streamChan:
            if !ok {
                pipeWriter.Close()
                return
            }
            pipeWriter.Write(audioData)
        case <-ctx.Done():
            return
        }
    }
}()

// Sử dụng music_player để phát âm thanh từ Pipe
audioChan, err := play_music.PlayMusicFromPipe(ctx, pipeReader, ...)
```

#### Cơ chế xử lý lỗi

**Thử lại khi timeout**:

```go
if err != nil {
    // Nếu là lỗi timeout, thử lại
    if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
        log.Warnf("Đọc tài nguyên bị timeout, đang thử lại...")
        time.Sleep(1 * time.Second)
        continue
    }
    return fmt.Errorf("Đọc tài nguyên thất bại: %v", err)
}
```

**Hủy context**:

```go
select {
case <-ctx.Done():
    log.Debugf("Việc đọc tài nguyên đã bị hủy")
    return nil
case streamChan <- rawAudioData:
    // Gửi dữ liệu bình thường
}
```

#### Đặc điểm của cơ chế phân trang

- **Tối ưu bộ nhớ**: Đọc phân trang tránh việc phải tải toàn bộ file lớn vào bộ nhớ cùng lúc
- **Xử lý theo luồng**: Vừa đọc vừa phát, hỗ trợ luồng âm thanh thời gian thực
- **Tự động kết thúc**: Phát hiện cờ `McpReadResourceStreamDoneFlag` để xác định đã đọc xong
- **Khôi phục khi lỗi**: Hỗ trợ thử lại khi timeout và hủy context
- **Phát thời gian thực**: Sử dụng cơ chế Pipe để vừa đọc vừa phát
- **Kiểm soát timeout**: Mỗi lần đọc phân trang đều có giới hạn timeout 30 giây

#### Tham số cấu hình

- **McpReadResourcePageSize**: Kích thước phân trang, mặc định 100KB (100 \* 1024)
- **McpReadResourceStreamDoneFlag**: Cờ kết thúc luồng, là `"[DONE]"`
- **Timeout đọc**: Thời gian timeout cho mỗi lần đọc phân trang, mặc định 30 giây
- **Cơ chế thử lại**: Tự động thử lại khi timeout, khoảng cách giữa các lần thử là 1 giây

**Kịch bản sử dụng**:

- Phát file âm thanh lớn
- Xử lý tài nguyên streaming media
- Truy cập tài nguyên qua mạng
- Phát luồng âm thanh thời gian thực

### 3. Nội dung văn bản (TextContent)

**Loại**: `mcp_go.TextContent`

**Đặc điểm**:

- Nội dung văn bản thuần túy
- Được tích lũy vào tin nhắn phản hồi
- Không dừng các bước xử lý tiếp theo

**Quy trình xử lý**:

```go
if textContent, ok := content.(mcp_go.TextContent); ok {
    mcpContent += textContent.Text
}
```

**Kịch bản sử dụng**:

- Trả về kết quả truy vấn
- Hiển thị thông tin trạng thái
- Hiển thị thông báo lỗi

### 4. Nội dung tài nguyên Blob (BlobResourceContents)

**Loại**: `mcp_go.BlobResourceContents`

**Đặc điểm**:

- Nội dung dữ liệu nhị phân
- Mã hóa Base64
- Hỗ trợ xử lý theo luồng

**Quy trình xử lý**:

```go
if audioContent, ok := content.(mcp_go.BlobResourceContents); ok {
    rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Blob)
    // Kiểm tra có phải cờ kết thúc hay không
    if string(rawAudioData) == McpReadResourceStreamDoneFlag {
        return nil
    }
    // Gửi vào channel luồng âm thanh
    streamChan <- rawAudioData
}
```

## 🏗️ Hệ thống phản hồi có cấu trúc

### Phân loại các kiểu phản hồi

Chương trình hỗ trợ bốn kiểu phản hồi chính:

#### 1. Phản hồi dạng hành động (MCPActionResponse)

- **Công dụng**: Thực hiện một hành động cụ thể, ví dụ phát nhạc, thoát hội thoại
- **Tính dừng luồng**: Có thể cấu hình, thường sẽ dừng các bước xử lý LLM tiếp theo
- **Cờ điều khiển**: `FinalAction`, `NoFurtherResponse`, `SilenceLLM`

#### 2. Phản hồi dạng âm thanh (MCPAudioResponse)

- **Công dụng**: Phát tài nguyên âm thanh
- **Tính dừng luồng**: Thường sẽ dừng các bước xử lý tiếp theo
- **Đặc điểm**: Chứa dữ liệu âm thanh và thông tin phát

#### 3. Phản hồi dạng nội dung (MCPContentResponse)

- **Công dụng**: Trả về dữ liệu truy vấn, thông tin trạng thái
- **Tính dừng luồng**: Không dừng các bước xử lý tiếp theo
- **Đặc điểm**: Chứa dữ liệu và gợi ý hiển thị

#### 4. Phản hồi dạng lỗi (MCPErrorResponse)

- **Công dụng**: Xử lý lỗi một cách thống nhất
- **Tính dừng luồng**: Không dừng các bước xử lý tiếp theo
- **Đặc điểm**: Chứa mã lỗi và gợi ý xử lý

### Interface xử lý phản hồi

```go
type MCPResponse interface {
    GetType() MCPResponseType
    GetSuccess() bool
    IsTerminal() bool // Quan trọng: xác định có dừng các bước xử lý LLM tiếp theo hay không
    ToJSON() (string, error)
    GetContent() []mcp_go.Content
}
```

## 🔄 Chi tiết quy trình xử lý

### 1. Thực thi lệnh gọi công cụ

```go
fcResult, err := tool.InvokableRun(toolCtx, toolCall.Function.Arguments)
```

### 2. Phân tích kết quả

```go
// Thử phân tích kết quả công cụ local
if mcpResp, ok := l.handleLocalToolResult(fcResult); ok {
    contentList = mcpResp.GetContent()
} else if toolCallResult, ok := l.handleToolResult(fcResult); ok {
    contentList = toolCallResult.Content
}
```

> `handleToolResult` **không còn yêu cầu giá trị trả về của công cụ bắt buộc phải là JSON**.
>
> - Nếu giá trị trả về là JSON `CallToolResult` chuẩn của MCP, sẽ được phân tích theo nội dung có cấu trúc.
> - Nếu giá trị trả về là một chuỗi văn bản bình thường, sẽ tự động được bọc thành `TextContent` để tiếp tục quy trình xử lý.  
>   Nhờ vậy, cả công cụ văn bản thông thường và công cụ MCP có cấu trúc đều được xử lý một cách thống nhất.

### 3. Xử lý theo loại nội dung

```go
for _, content := range contentList {
    switch content.(type) {
    case mcp_go.AudioContent:
        // Xử lý nội dung âm thanh
    case mcp_go.ResourceLink:
        // Xử lý liên kết tài nguyên
    case mcp_go.TextContent:
        // Xử lý nội dung văn bản
    }
}
```

### 4. Kiểm soát xử lý tiếp theo

```go
if invokeToolSuccess && !shouldStopLLMProcessing {
    l.DoLLmRequest(ctx, nil, l.einoTools, true)
}
```

## 📊 Bảng so sánh các loại nội dung

| Loại nội dung            | Tính dừng luồng | Cách xử lý                      | Kịch bản sử dụng         | Công cụ ví dụ |
| ------------------------ | --------------- | ------------------------------- | ------------------------ | ------------- |
| **AudioContent**         | Dừng            | Phát trực tiếp                  | File âm thanh nhỏ        | play_music    |
| **ResourceLink**         | Dừng            | Đọc phân trang + phát streaming | File lớn/streaming media | music_player  |
| **TextContent**          | Không dừng      | Tích lũy văn bản                | Truy vấn thông tin       | get_datetime  |
| **BlobResourceContents** | Dừng            | Xử lý theo luồng                | Dữ liệu luồng âm thanh   | audio_stream  |

## 🎯 Thực hành tốt nhất

### 1. Đề xuất khi triển khai công cụ

- **Công cụ âm thanh**: Trả về `AudioContent` hoặc `ResourceLink`
- **Công cụ truy vấn**: Trả về `TextContent`
- **Công cụ hành động**: Sử dụng hệ thống phản hồi có cấu trúc

### 2. Tối ưu hiệu năng

- File lớn dùng `ResourceLink` để xử lý phân trang, hỗ trợ phát streaming
- File âm thanh nhỏ dùng trực tiếp `AudioContent`, giảm chi phí mạng
- Nội dung văn bản tránh quá dài, ảnh hưởng đến tốc độ phản hồi
- Sử dụng cơ chế Pipe để vừa đọc vừa phát, nâng cao trải nghiệm người dùng

### 3. Xử lý lỗi

- Sử dụng `MCPErrorResponse` để thống nhất định dạng lỗi
- Cung cấp mã lỗi và gợi ý có ý nghĩa
- Đảm bảo tính tương thích ngược

## 🔧 Tham số cấu hình

### Cấu hình phân trang

- `McpReadResourcePageSize`: Kích thước phân trang khi đọc tài nguyên, mặc định 100KB (100 \* 1024)
- `McpReadResourceStreamDoneFlag`: Cờ kết thúc luồng, là `"[DONE]"`
- **Timeout đọc**: Thời gian timeout cho mỗi lần đọc phân trang, mặc định 30 giây
- **Cơ chế thử lại**: Tự động thử lại khi timeout, khoảng cách giữa các lần thử là 1 giây

### Cấu hình âm thanh

- `OutputAudioFormat.SampleRate`: Tần số lấy mẫu âm thanh đầu ra
- `OutputAudioFormat.FrameDuration`: Thời lượng khung hình âm thanh đầu ra
- **Định dạng âm thanh**: Tự động nhận diện dựa theo `resourceLink.MIMEType`

## 📝 Hướng dẫn mở rộng

### Thêm loại nội dung mới

1. Định nghĩa loại nội dung mới trong package `mcp_go`
2. Thêm logic xử lý loại mới trong `handleToolCallResponse`
3. Triển khai hàm xử lý tương ứng
4. Cập nhật tài liệu và test

### Tùy chỉnh loại phản hồi

1. Kế thừa `MCPResponseBase`
2. Triển khai interface `MCPResponse`
3. Thêm logic phân tích trong `ParseMCPResponse`
4. Cung cấp hàm khởi tạo (constructor) tiện lợi

## 🎵 MCP Audio Server (Repository độc lập)

### Tổng quan

MCP Audio Server đã được tách thành một repository độc lập, khuyến nghị chạy và debug MCP Server loại âm thanh thông qua dự án độc lập này. Phần này trong tài liệu hiện tại chủ yếu giải thích cách nó tương thích về giao thức với dịch vụ chính.

### Chức năng cốt lõi

#### 1. Công cụ phát nhạc

- **Tên công cụ**: `musicPlayer`
- **Chức năng**: Tìm kiếm và phát nhạc
- **Trả về**: Liên kết tài nguyên âm thanh dạng `ResourceLink`

#### 2. Mẫu tài nguyên âm thanh

- **Định dạng URI**: `resource://read_from_http`
- **Chức năng**: Hỗ trợ đọc phân trang dữ liệu âm thanh, truyền tham số qua Arguments
- **Tham số**: url (URL nhạc thực tế), start (vị trí bắt đầu), end (vị trí kết thúc)
- **Trả về**: Dữ liệu âm thanh dạng `BlobResourceContents`

### Đặc tính chính

- **Đọc phân trang**: Hỗ trợ xử lý theo luồng cho file lớn
- **Yêu cầu HTTP Range**: Thực hiện lấy dữ liệu âm thanh theo từng đoạn
- **Xử lý lỗi**: Xử lý các trường hợp bất thường như mã trạng thái 416
- **Thử lại khi timeout**: Tự động thử lại khi gặp lỗi timeout, cách nhau 1 giây
- **Hủy context**: Hỗ trợ hủy việc đọc tài nguyên một cách an toàn (graceful)
- **Mã hóa Base64**: Truyền tham số URL nhạc một cách an toàn
- **Hỗ trợ đa transport**: Hai phương thức truyền tải stdio và HTTP
- **Phát thời gian thực**: Sử dụng cơ chế Pipe để vừa đọc vừa phát

### Cách sử dụng

```bash
# Lấy và vào repository độc lập
git clone https://github.com/quoctho228/mcp_audio_server.git
cd mcp_audio_server

# Khởi động server
go run .

# Gọi công cụ
{
  "name": "musicPlayer",
  "arguments": {"query": "Châu Kiệt Luân"}
}
```

Dự án độc lập này minh họa cách xây dựng công cụ MCP hỗ trợ xử lý tài nguyên âm thanh, có thể dùng làm template tham khảo để phát triển các công cụ liên quan đến âm thanh khác. Hướng dẫn sử dụng đầy đủ hơn có thể tham khảo tại `doc/mcp_audio_example.md`.

---

_Tài liệu này phản ánh toàn bộ các loại nội dung trả về khi gọi công cụ mà chương trình hiện đang hỗ trợ._
