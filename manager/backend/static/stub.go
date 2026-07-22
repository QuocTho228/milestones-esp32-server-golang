//go:build !embed_ui

package static

import "embed"

// FS sẽ để trống nếu chưa bật embed_ui; trong giai đoạn phát triển không gắn kết tài nguyên tĩnh của giao diện web.
var FS = embed.FS{}
