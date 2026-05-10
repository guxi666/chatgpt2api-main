# 使用 GHCR 在 Ubuntu 宝塔部署（含邮箱注册与易支付）

## 1) 本地发布镜像

在项目根目录执行：

```powershell
.\scripts\release-ghcr.ps1 -GitHubUser <你的GitHub用户名> -ImageName chatgpt2api -Tag billing-v1
```

发布成功后会得到镜像地址，例如：

`ghcr.io/<你的GitHub用户名>/chatgpt2api:billing-v1`

## 2) 服务器部署

把项目放到服务器（例如 `/www/wwwroot/chatgpt2api`），并确保有 `.env`：

```bash
cd /www/wwwroot/chatgpt2api
cp .env.example .env
```

编辑 `.env` 至少配置：

```env
CHATGPT2API_AUTH_KEY=请改成强密码
CHATGPT2API_BASE_URL=https://你的域名

CHATGPT2API_IMAGE_PRICE_CENTS=8
CHATGPT2API_EMAIL_ALLOWED_DOMAINS=qq.com,163.com,126.com,gmail.com,outlook.com,hotmail.com,icloud.com,yahoo.com,foxmail.com,sina.com

CHATGPT2API_YIPAY_ENABLED=true
CHATGPT2API_YIPAY_PID=你的PID
CHATGPT2API_YIPAY_KEY=你的KEY
CHATGPT2API_YIPAY_SUBMIT_URL=https://你的易支付域名/submit.php
CHATGPT2API_YIPAY_NOTIFY_URL=https://你的域名/api/pay/yipay/notify
CHATGPT2API_YIPAY_RETURN_URL=https://你的域名/image
CHATGPT2API_YIPAY_SITE_NAME=chatgpt2api
```

执行部署脚本：

```bash
chmod +x scripts/deploy-server.sh
./scripts/deploy-server.sh ghcr.io/<你的GitHub用户名>/chatgpt2api:billing-v1 /www/wwwroot/chatgpt2api
```

## 3) 宝塔反向代理

- 站点域名反代到 `127.0.0.1:3000`
- 开启 HTTPS

## 4) 易支付回调地址

在易支付后台设置异步通知地址：

`https://你的域名/api/pay/yipay/notify`

## 5) 快速验收

1. `POST /auth/register` 注册邮箱用户
2. `POST /api/pay/orders` 创建充值订单并支付
3. `GET /api/wallet` 查看余额到账
4. 调用 `/v1/images/generations` 验证每次扣 `8` 分
