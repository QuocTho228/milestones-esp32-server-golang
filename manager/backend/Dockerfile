FROM golang:1.21-alpine AS builder

WORKDIR /app

# Sao chép file go mod
COPY go.mod go.sum ./
RUN go mod download

# Sao chép mã nguồn
COPY . .

# Build ứng dụng
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Giai đoạn chạy (runtime stage)
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Sao chép file thực thi (binary) từ giai đoạn build
COPY --from=builder /app/main .
COPY --from=builder /app/config ./config

# Thiết lập múi giờ
ENV TZ=Asia/Shanghai

EXPOSE 8080

CMD ["./main"]