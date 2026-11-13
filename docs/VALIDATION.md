# 验证记录

本地核验时间：2026-11-14T02:00:00+08:00。项目仓库为 [Alumos/tnr-atlas](https://github.com/Alumos/tnr-atlas)，CI 与镜像运行结果以 [GitHub Actions](https://github.com/Alumos/tnr-atlas/actions) 为准。

## 代码检查

| 检查 | 结果 |
| --- | --- |
| TypeScript + ESLint：`npm run check` | 通过 |
| 前端生产构建：`npm run build` | 通过 |
| 静态 gzip 预生成：`node scripts/precompress.mjs` | 通过，支持工作目录中含中文和空格 |
| Go：`go test -race ./...` | 通过，9 个测试函数，含 5 个图床契约子用例 |
| Go：`go vet ./...` | 通过 |
| Go：`go build ./cmd/server` | 通过；仅编译代码，未构建 Docker 镜像 |
| 本地 QA 图床脚本：`node --check scripts/preview-oneimg.mjs` | 通过 |
| 生产 Compose 与部署文档中的完整 Compose | 内容一致；仅 tnr/oneimg 两个服务，无 build 字段 |
| Compose、开发 Compose、GitHub 工作流 YAML | Ruby 标准 YAML 解析通过；尚未执行 Docker Compose 环境校验 |

本机没有 Docker，未安装系统工具。检查使用已有 Node v26.13.0、Go go1.27.3 darwin/arm64，缓存写入 `/tmp`。项目提供 Node 24、Go 1.27 的容器检查流程和 GitHub Actions；这些容器流程仍需在具有 Docker 的环境实际运行。

测试覆盖：

- 未登录录入拒绝、志愿者与管理员权限分离、跨来源写入拒绝。
- 公共访问码轮换、志愿者停用和持久化设置；初始化环境变量不会覆盖后台保存的公共码。
- JPEG、PNG 原始字节不变；按实际输入格式设置扩展名和 MIME；每张原图只调用一次图床上传。
- 图床返回主图/缩略图和尺寸元数据的使用；`folder_id` 转发及返回值核对。
- 图床未转 WebP、没有缩略图、缩略图 URL 无效或目录不匹配时拒绝发布档案。
- 多图部分上传失败后，不发布半份档案，已知图床文件记录到保留审计。
- 猫咪创建、修改、删除，照片追加和移除，审计及图床文件保留；没有调用图床删除接口。
- 搜索通配符转义、SPA 后台路径、静态文件缓存及 gzip 协商。

## ego lite 浏览器核验

浏览器访问本机回环预览；测试数据使用临时 SQLite。没有访问真实生产图床或修改 Mac 网络配置。

完成了独立志愿者创建/登录、普通 JPEG 快速录入、总览/详情加载、猫咪改名和绝育状态修改、追加照片、设置封面、移除照片、更换公共码、停用志愿者、删除测试猫和查看审计快照。志愿者停用后会话失效，猫咪删除后公开统计归零。测试资料均未写入生产数据库。

录入时图床模拟器收到的 JPEG 与选择的原始 JPEG 做 SHA-256 比较，字节一致；一张照片仅收到一次上传。总览资源记录只含图床返回的 `-thumb.webp`，打开详情后才出现主图请求，列表图片均有 `loading="lazy"`。修复了后台封面按钮被移除按钮遮挡的问题，并重新核验封面选择及前端构建。

| 页面 / 宽度 | 核验结果 |
| --- | --- |
| 总览 390px | 两列卡片，无横向溢出 |
| 总览 768px | 两列卡片，无横向溢出 |
| 总览 1440px | 四列卡片，无横向溢出 |
| 总览 1920px | 四列卡片，无横向溢出 |
| 大屏 1920px | 进入浏览器全屏、隐藏介绍区、六列卡片，无横向溢出 |
| 后台 390px | 表单和列表适配；审计表在区域内横向滚动，页面无横向溢出 |
| 深色主题 | 切换并完成视觉检查 |

截图为明确标注的演示页面，猫咪资料为虚构，样图来自 Unsplash。演示图像大小参数与实际 oneimg 的处理策略互不相关。

- [桌面总览](screenshots/desktop.png)
- [手机总览](screenshots/mobile.png)
- [深色主题](screenshots/dark.png)
- [大屏](screenshots/display.png)
- [手机后台](screenshots/admin-mobile.png)

## 尚需部署环境验证

`scripts/preview-oneimg.mjs` 是本地接口测试替身：保存收到的原图，返回固定的有效 WebP 样图与独立 URL。它不实现图片转换，不能证明真实图床转码后的画质、体积、方向处理或压缩耗时。它不会进入生产启动链路。

官方 oneimg 源码已核对，但未在本机运行官方 Docker 镜像。真实镜像的上传配置、目录支持、公开 URL、WebP 输出、缩略图、反向代理、卷权限和服务器容量，应按 [部署手册第 7 节](DEPLOYMENT.md#7-首次验收) 验收。当前没有真实服务器吞吐量或加载速度基准，不能承诺具体响应时间。

GitHub Actions 在 `main` 推送后执行检查并发布 GHCR 镜像；本文的本地检查结果不等同于 CI 或容器检查结果。没有在本机或 1Panel 构建任何部署镜像。
