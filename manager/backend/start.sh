#!/bin/bash

echo "=== Script khởi động backend hệ thống quản lý Milestones ==="

# Kiểm tra tham số
if [ "$1" = "help" ] || [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "Cách sử dụng:"
    echo "  ./start.sh                    # Sử dụng file cấu hình mặc định"
    echo "  ./start.sh dev                # Sử dụng cấu hình môi trường development"
    echo "  ./start.sh prod               # Sử dụng cấu hình môi trường production"
    echo "  ./start.sh custom config.json # Sử dụng file cấu hình tùy chỉnh"
    echo "  ./start.sh reset              # Reset lại database và dùng cấu hình mặc định"
    echo "  ./start.sh reset-dev          # Reset lại database và dùng cấu hình môi trường development"
    echo "  ./start.sh help               # Hiển thị thông tin trợ giúp"
    exit 0
fi

# Thiết lập đường dẫn file cấu hình
CONFIG_FILE="manager/backend/config/config.json"

RESET_DB=""

case "$1" in
    "dev")
        CONFIG_FILE="manager/backend/config/config.dev.json"
        echo "Sử dụng cấu hình môi trường development: $CONFIG_FILE"
        ;;
    "prod")
        CONFIG_FILE="manager/backend/config/config.prod.json"
        echo "Sử dụng cấu hình môi trường production: $CONFIG_FILE"
        ;;
    "reset")
        RESET_DB="-reset-db"
        echo "Reset lại database và dùng cấu hình mặc định: $CONFIG_FILE"
        ;;
    "reset-dev")
        CONFIG_FILE="manager/backend/config/config.dev.json"
        RESET_DB="-reset-db"
        echo "Reset lại database và dùng cấu hình môi trường development: $CONFIG_FILE"
        ;;
    "custom")
        if [ -z "$2" ]; then
            echo "Lỗi: vui lòng chỉ định đường dẫn file cấu hình"
            echo "Cách sử dụng: ./start.sh custom config.json"
            exit 1
        fi
        CONFIG_FILE="$2"
        echo "Sử dụng cấu hình tùy chỉnh: $CONFIG_FILE"
        ;;
    "")
        echo "Sử dụng cấu hình mặc định: $CONFIG_FILE"
        ;;
    *)
        echo "Tham số không xác định: $1"
        echo "Dùng './start.sh help' để xem trợ giúp"
        exit 1
        ;;
esac

# Kiểm tra file cấu hình có tồn tại hay không
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Lỗi: file cấu hình không tồn tại: $CONFIG_FILE"
    exit 1
fi

# Vào thư mục backend
cd manager/backend

# Cài đặt dependency
echo "Đang cài đặt dependency Go..."
go mod tidy

# Khởi động service
echo "Đang khởi động service..."
if [ -n "$RESET_DB" ]; then
    echo "Cảnh báo: database sẽ bị reset, toàn bộ dữ liệu sẽ bị xóa!"
    read -p "Bạn có chắc chắn muốn tiếp tục không? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "Đã hủy thao tác"
        exit 0
    fi
    go run main.go -config="../../$CONFIG_FILE" $RESET_DB
else
    go run main.go -config="../../$CONFIG_FILE"
fi