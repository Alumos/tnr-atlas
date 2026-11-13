# 猫咪档案 · TNR Atlas

为公益 TNR 计划记录社区猫咪。React + shadcn/ui 界面由 Node 构建，Go 同时提供页面和 API，SQLite 保存档案、志愿者与审计记录。

**生产只部署两个容器：TNR 平台 + 官方原版 oneimg。** 无需独立前端、Nginx、数据库或图片处理服务。本项目的镜像由 GitHub Actions 构建并发布到 GHCR。

## 已实现

- 猫咪姓名、性别、毛色、相册、绝育状态、绝育日期（选填）、描述、上传人；「未知」「无法定义」可用于尚未确认的信息。
- 快速录入侧栏，照片选择/拖入与预览；1–6 张，每张不超过 10 MiB，相册总计不超过 50 MiB。
- 首页「录入新猫咪」采用 56px 大按钮，顶部快速录入入口为 48px，适配手机触控。
- 上传普通 JPEG、PNG 或 WebP；Go 校验格式、体积和像素数，原始图片字节只转发一次。官方 oneimg 负责压缩、转 WebP 和生成缩略图；浏览器懒加载图床缩略图，打开详情才取主图。
- 公开总览、搜索、性别/毛色/绝育状态筛选、分页、紧凑视图、主题切换、响应式布局和全屏大屏模式。
- `/admin` 管理公共访问码、独立志愿者账号、猫咪资料和照片；变更记录写入审计日志。
- 停用志愿者、重置个人访问码或修改公共访问设置后，相应旧会话立即失效。
- 直接使用 oneimg 官方上传 API，并支持上传到预先创建的单层文件夹。
- 兼容官方上传响应中的相对图片路径；`tnr` 的 `ONEIMG_PUBLIC_ORIGIN` 用于生成浏览器可访问的主图和缩略图 URL，内部上传地址仍使用 `http://oneimg:8080`。

## 图床行为

官方版的上传 Token 不能创建文件夹或标签，也不能删除图片。已核验源码中的文件夹只有一层，因此不提供 `NTR/猫咪名字/` 这种真实的父子目录。

推荐在 oneimg **手工创建一次 `NTR` 文件夹**，将其 `folder_id` 配置到 TNR，并开启「图片压缩」「保存 WebP」「缩略图」「保存源文件名」。后续所有录入自动归入此目录，图床处理后的主图文件名类似：

```text
NTR/
  NTR-小橘-7f1c9a2e4b6d80aa-90ab12cd34ef56ab.webp
```

固定相册编号区分同名猫咪；末尾随机编号区分同一相册内的照片。平台内每只猫都有自己的相册。修改名字后原照片保持关联，新上传文件使用新名字和原相册编号。

每张原图对应一条 oneimg 图片记录，主图和缩略图均由图床管理。当前核验版本的缩略图限制在 300×300 内，详情使用图床返回主图；TNR 不承诺固定主图尺寸。如果图床未返回 WebP 主图或独立缩略图，录入会明确报错并记录已知保留文件，供管理员检查设置。

**从 TNR 移除照片或删除猫咪档案，oneimg 文件继续保留。** 管理员可在审计快照中找到原图片 URL 和 oneimg 图片 ID。图床不需要修改版镜像或扩展。

## 部署和开发

- [完整 1Panel / Docker Compose 部署文档](docs/DEPLOYMENT.md)
- [架构、API 和安全边界](docs/ARCHITECTURE.md)
- [验证记录](docs/VALIDATION.md)
- [第三方来源](docs/SOURCES.md)

项目仓库：[Alumos/tnr-atlas](https://github.com/Alumos/tnr-atlas)。推送代码后，Actions 自动执行检查并发布 `ghcr.io/alumos/tnr-atlas`，支持 `linux/amd64` 与 `linux/arm64`。

首个版本 **[v0.1.0](https://github.com/Alumos/tnr-atlas/releases/tag/v0.1.0)** 已发布，[代码检查及双架构运行检查均通过](https://github.com/Alumos/tnr-atlas/actions/runs/43491838328)。`.env.example` 默认使用：

```text
ghcr.io/alumos/tnr-atlas:latest
```

固定首个版本可使用 `ghcr.io/alumos/tnr-atlas:0.1.0`。使用 `latest` 时，升级只需拉取并重新创建 TNR 容器，不必填写提交编号。完整命令和两容器编排见部署文档。

开发优先使用官方工具容器，避免在主机安装系统包：

```bash
docker compose -f compose.dev.yaml run --rm web-check
docker compose -f compose.dev.yaml run --rm api-check
```

如果已经装有 Node 与 Go，亦可在各子目录运行：

```bash
cd web
npm ci
npm run check
npm run build
node scripts/precompress.mjs
```

```bash
cd api
go test -race ./...
go vet ./...
go build ./cmd/server
```

启动 API 需要设置 `TNR_ACCESS_CODE`、`TNR_ADMIN_PASSWORD` 和 `PUBLIC_ORIGIN`；环境变量含义见部署文档。`npm run dev` 的 Vite 端口为 5173，API 为 8081，开发时将 `PUBLIC_ORIGIN` 设为 `http://127.0.0.1:5173`。

生产数据库初始为空。`/?demo=1` 是明确标注的界面演示，使用虚构档案和 Unsplash 图片，不写入数据库。
