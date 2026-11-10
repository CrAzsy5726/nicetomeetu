# 构建阶段
FROM node:24-alpine AS build
WORKDIR /app

COPY package*.json ./
# 国内 npm 镜像，避免 registry.npmjs.org 慢/超时
RUN npm config set registry https://registry.npmmirror.com && npm install

COPY . .
RUN npm run build

# 运行阶段
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
