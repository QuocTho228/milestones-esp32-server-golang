#!/usr/bin/env bash

set -euo pipefail

# Dùng cho gói phát hành (release package) trên macOS: chuyển đường dẫn tuyệt đối (rpath)
# trỏ tới mã nguồn trên máy dev thành đường dẫn tương đối so với file thực thi.
# Cấu trúc thư mục áp dụng:
#   ./milestones_server
#   ./ten-vad/lib/macOS/ten_vad.framework

if [[ $# -ne 1 ]]; then
  echo "Cách dùng: $0 <đường dẫn tệp nhị phân milestones_server>" >&2
  exit 1
fi

BIN_PATH="$1"
TARGET_RPATH="@executable_path/ten-vad/lib/macOS"

if [[ ! -f "$BIN_PATH" ]]; then
  echo "Tệp nhị phân không tồn tại: $BIN_PATH" >&2
  exit 1
fi

if ! command -v otool >/dev/null 2>&1; then
  echo "Thiếu otool, vui lòng cài đặt Xcode Command Line Tools" >&2
  exit 1
fi

if ! command -v install_name_tool >/dev/null 2>&1; then
  echo "Thiếu install_name_tool, vui lòng cài đặt Xcode Command Line Tools" >&2
  exit 1
fi

CURRENT_RPATHS=()
while IFS= read -r line; do
  CURRENT_RPATHS+=("$line")
done < <(
  otool -l "$BIN_PATH" | awk '
    $1 == "cmd" && $2 == "LC_RPATH" { in_rpath = 1; next }
    in_rpath && $1 == "path" { print $2; in_rpath = 0 }
  '
)

if [[ ${#CURRENT_RPATHS[@]} -eq 0 ]]; then
  echo "Không phát hiện LC_RPATH nào, chuẩn bị ghi trực tiếp rpath đích"
fi

for rpath in "${CURRENT_RPATHS[@]}"; do
  if [[ "$rpath" == "$TARGET_RPATH" ]]; then
    continue
  fi
  install_name_tool -delete_rpath "$rpath" "$BIN_PATH" 2>/dev/null || true
done

if ! otool -l "$BIN_PATH" | grep -Fq "path $TARGET_RPATH "; then
  install_name_tool -add_rpath "$TARGET_RPATH" "$BIN_PATH"
fi

echo "Đã ghi rpath: $TARGET_RPATH"