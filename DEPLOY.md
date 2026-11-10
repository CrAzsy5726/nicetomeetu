# NiceToMeetU 部署指南（阿里云 ECS · Ubuntu 24.04 · 2核2G）

按顺序执行，每一步的命令都可直接复制粘贴。遇到报错把输出贴出来即可。

## 0. 前置准备（阿里云控制台）

1. 登录阿里云控制台 → 找到你的 ECS 实例 → **安全组** → 添加入方向规则，放行：
   - **80**（HTTP）
   - **443**（HTTPS，如果之后要配域名的话）
   - 协议类型选「自定义 TCP」，授权对象 `0.0.0.0/0`
2. 用 `ssh root@你的服务器IP` 登录服务器（或用阿里云网页版「远程连接」）。

---

## 1. 安装 Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
```

> 执行完 `usermod` 后**退出重新登录**一次 SSH，让用户组生效（否则后面 docker 命令要加 sudo）。

验证：`docker --version` 和 `docker compose version` 能输出版本号即可。

---

## 2. 设置 2GB swap（防止内存不足 OOM）

2核2G 跑 Milvus 比较紧，先加 2G swap 兜底：

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

验证：`free -h` 能看到 Swap 那行有 2G。

---

## 3. 上传项目代码

二选一：

**方式 A（推荐）：推到 Git 仓库，再 clone**

在你本地电脑上：
```bash
cd NiceToMeetU
git add .
git commit -m "deploy"
git remote add origin <你的仓库地址>   # GitHub 或 Gitee（国内用 Gitee 更快）
git push -u origin master
```

在服务器上：
```bash
git clone <你的仓库地址> NiceToMeetU
cd NiceToMeetU
```

**方式 B：直接 scp 上传**

在你本地电脑上（Windows 的 PowerShell 或 WSL）：
```bash
scp -r NiceToMeetU root@你的服务器IP:/root/
```

---

## 4. 配置密钥

在服务器上，进入项目目录：

```bash
cd NiceToMeetU
cp .env.production.example .env
nano .env   # 或者 vi .env
```

填入你的两个密钥：
- `DEEPSEEK_API_KEY`
- `ZHIPU_API_KEY`

`POSTGRES_PASSWORD` 建议改成自己的密码（可选）。

---

## 5. 构建并启动

```bash
docker compose -f docker-compose.prod.yml up -d --build
```

> 第一次构建需要拉镜像 + 编译前后端，大约 5~15 分钟。`-d` 是后台运行，构建过程可以看日志：`docker compose -f docker-compose.prod.yml logs -f`。

---

## 6. 验证是否成功

```bash
# 看所有容器状态（都应是 Up/healthy）
docker compose -f docker-compose.prod.yml ps
```

然后浏览器打开 `http://你的服务器IP`，应该能看到登录页。注册一个账号、选个伙伴、发一句消息，确认：
1. 聊天能正常流式回复；
2. 聊几句后顶栏「已了解你 N 件事」会增长（说明画像提取 + Milvus 正常）。

---

## 7. 常用运维命令

```bash
# 查看日志
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f milvus

# 重启
docker compose -f docker-compose.prod.yml restart

# 停止
docker compose -f docker-compose.prod.yml down

# 更新代码后重新部署
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

---

## 8. 内存如果还是紧张（备用方案）

如果 `docker stats` 看到 Milvus 频繁重启、或记忆功能偶尔报错，说明内存确实不够。两个方向：

1. 把 `docker-compose.prod.yml` 里 milvus 的 `mem_limit: 768m` 再调低到 `512m`（数据量小，通常够）。
2. 彻底方案：把 Milvus 换成 PostgreSQL 的 `pgvector` 扩展，省掉整个 Milvus 全家桶（约省 1G 内存），需要改一部分后端代码——这个我可以帮你做。

---

## 9. 配域名 + HTTPS（可选，之后做）

需要先有一个域名，并把它的 A 记录解析到服务器 IP。之后有两种做法：

- **简单**：用 Caddy 反代（自动申请 Let's Encrypt 证书）。
- **传统**：宿主机装 nginx + certbot。

这块等你有了域名再跟我说，我按你的域名把配置写好。
