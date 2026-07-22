FROM node:18-alpine AS builder

WORKDIR /app

# Sao chép các tệp package.
COPY package*.json ./
RUN npm ci

# Sao chép mã nguồn.
COPY . .

# Xây dựng ứng dụng.
RUN npm run build

# Giai đoạn chạy.
FROM nginx:alpine

# Sao chép kết quả sau khi xây dựng.
COPY --from=builder /app/dist /usr/share/nginx/html

# Sao chép cấu hình Nginx.
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]