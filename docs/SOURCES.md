# 来源与核验基线

核验日期：2026-11-14（Asia/Taipei）。本项目没有修改 oneimg 源码，也没有向真实图床发起写入操作。

## oneimg 官方源码

研究克隆来自 [onexru/oneimg](https://github.com/onexru/oneimg)，提交 [`0f5a2d4ee07ba2615739ffc9974aaccddb44ee72`](https://github.com/onexru/oneimg/tree/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72)，提交日期 2026-11-09。

- [官方 README](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/README.md)：官方镜像 `onexru/oneimg-oneimg`、环境变量、数据卷和默认凭据行为。
- [官方 Compose](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/docker-compose.yml)：数据、上传目录及部署配置。
- [API 路由](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/routes/routes.go)：上传、目录、图片管理端点。
- [认证中间件](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/middlewares/AuthMiddleware.go)：Bearer Token 支持与接口白名单；无 Token 删除/建目录权限。
- [上传控制器](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/controllers/images/ImageUpload.go)、[上传字段校验](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/controllers/images/UploadLimits.go)、[返回结构](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/interfaces/image.go)：`images[]`、已有标签关联、`data.files[]`。
- [存储上传返回值](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/utils/uploads/storage_upload.go)、[兼容上传处理](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/controllers/images/LegacyUpload.go)、[公开 URL 策略](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/utils/publicurl/publicurl.go)：本地存储的 `url` 和 `thumbnail_url` 可直接返回 `/uploads/…` 相对路径；`APP_URL` 不保证将上传响应转换为绝对地址。TNR 需用单独配置的图床公开域名解析这些路径，不能要求官方响应始终为绝对 URL。
- [文件夹服务](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/services/gallery_folders.go)、[文件夹控制器](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/controllers/images/Folders.go)：单层目录、命名约束、200 个上限、`folder_id` 的归属检查。
- [图片处理](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/utils/images/images.go)、[上传设置](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/models/Settings.go)：图床压缩/转 WebP/生成缩略图，缩略图最大 300×300、质量 80；保存源文件名的设置影响最终文件名称。TNR 不自行转码。
- [认证凭据](https://github.com/onexru/oneimg/blob/0f5a2d4ee07ba2615739ffc9974aaccddb44ee72/backend/utils/authsecurity/credentials.go)：Token 默认有效期 30 天。

官方镜像 `latest` 是可变标签，不能仅凭仓库源码推断它永远与上述提交完全一致。首次部署需按部署文档完成一只测试猫的实际上传验收，再固定使用的官方镜像版本或 digest。

## GitHub Actions 运行时

2026-11-14 核对各 Action 官方发布和 `action.yml`，工作流使用 Node 24 运行时，固定版本见 `.github/workflows/ci.yml`。主要来源：[checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1)（2026-07-20）、[setup-node v7.1.0](https://github.com/actions/setup-node/releases/tag/v7.1.0)（2026-10-08）、[setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0)（2026-07-16）、[build-push-action v7.4.0](https://github.com/docker/build-push-action/releases/tag/v7.4.0)（2026-09-15）。检查和镜像构建固定使用 `ubuntu-24.04` Runner，避免 `ubuntu-latest` 自动迁移带来的环境变化。

## 前端与图片处理

- [shadcn/ui 官方文档](https://ui.shadcn.com/docs)：通过官方 CLI 获取组件，选择 Radix / new-york / neutral，检查组合和无障碍标题。
- [表单 Field](https://ui.shadcn.com/docs/components/radix/field)、[Select](https://ui.shadcn.com/docs/components/radix/select)、[Sheet](https://ui.shadcn.com/docs/components/radix/sheet)、[Alert Dialog](https://ui.shadcn.com/docs/components/radix/alert-dialog)：对应录入、筛选和删除确认。
- [Go image 包](https://pkg.go.dev/image)、[WebP 头部解析](https://pkg.go.dev/golang.org/x/image/webp)：TNR 仅校验输入格式和尺寸，不重新编码图片。

演示页面的照片来自 [Unsplash](https://unsplash.com/)，具体照片 URL 保存在 `web/src/lib/demo.ts`。示例猫咪姓名、状态和上传人均为虚构，不推断图片中的动物真实信息，不写入生产数据库。
