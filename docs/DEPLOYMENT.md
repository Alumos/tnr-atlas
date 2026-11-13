# TNR Atlas 与官方 oneimg：1Panel 部署手册

核验日期：2026-11-14（Asia/Taipei）。图床接口基于 oneimg 官方仓库提交 `0f5a2d4ee07ba2615739ffc9974aaccddb44ee72`，来源和版本限制见 [SOURCES.md](SOURCES.md)。

## 1. 最终部署内容

仅有两个应用容器：

| 服务 | 镜像 | 职责 |
| --- | --- | --- |
| `tnr` | GitHub Actions 构建的 `ghcr.io/alumos/tnr-atlas` | 页面、Go API、原图校验和转发、档案、志愿者、管理和审计 |
| `oneimg` | 官方 `onexru/oneimg-oneimg` | 图片压缩、转 WebP、生成缩略图、图床后台、存储和图片公开访问 |

无需修改或重新构建 oneimg。不部署独立 Node、数据库、图片处理、Nginx 或桥接容器。1Panel 已有的网站反向代理用于配置 HTTPS 域名。

普通 JPEG、PNG 或 WebP 图片从浏览器提交给 Go，校验后按原始字节上传到 oneimg，每张只上传一次。oneimg 负责压缩、转 WebP 和生成缩略图。浏览器通过 oneimg 返回的公开 URL 读取主图和缩略图，所以图床必须有浏览器可访问的 HTTPS 地址。

建议准备两个域名，例如：

- `https://cats.example.com`：TNR 平台。
- `https://img.example.com`：oneimg 图床及图片 URL。

域名和证书请在部署服务器的 1Panel 中配置。本项目不需要检查或修改开发用 Mac 的网络设置。

建议初始服务器配置至少 2 核、2 GiB 内存并预留图片磁盘空间。TNR 最多同时转发两个相册；oneimg 解码和压缩高像素图片会占用较多内存，实际容量应根据照片尺寸和并发做服务器端验收。

## 2. 创建 GitHub 仓库与构建镜像

项目仓库为 [Alumos/tnr-atlas](https://github.com/Alumos/tnr-atlas)。代码推送到 `main` 后，GitHub Actions 自动检查并发布镜像；部署服务器由你在 1Panel 中操作。

仓库中的 `.github/workflows/ci.yml` 会执行：

1. `npm ci`、TypeScript 和 ESLint 检查、前端生产构建。
2. Go race 测试、`go vet`、后端构建。
3. 检查通过后，GitHub 构建 `linux/amd64` 和 `linux/arm64` 的 **一个 TNR 镜像**并推送到 GHCR。

触发规则：`main` 分支推送、`v*` 标签、手工运行。PR 仅检查，不推送镜像。镜像地址由仓库名自动生成并转为小写；不会在你的 Mac 或 1Panel 上构建镜像。

本仓库的镜像产物为：

```text
ghcr.io/alumos/tnr-atlas:latest
ghcr.io/alumos/tnr-atlas:sha-完整提交编号
```

`main` 生成 `latest`；每次发布都有固定提交标签，正式运行建议用 `sha-...` 标签。发布 `v1.0.0` 标签还会生成 `1.0.0` 镜像标签。

GitHub Actions 使用仓库自带的 `GITHUB_TOKEN`，工作流已声明 `packages: write`，不需要把图床 Token 或任何生产密码上传到 GitHub。若组织策略禁止该权限，需要仓库管理员允许工作流发布 Packages。

第一次发布后，到 GitHub 对应 Package 设置中确认镜像可见性：

- **公开镜像**：1Panel 可直接拉取。
- **私有镜像**：在 1Panel 的镜像仓库凭据中配置 GHCR 和具有 `read:packages` 权限的凭据。不要把这个凭据写入 Compose 或提交到 Git。

oneimg 直接拉取官方镜像。官方镜像支持哪些架构应以实际镜像清单为准；如果部署服务器为 ARM，先确认官方发布支持该架构。

## 3. 在 1Panel 创建编排

在「容器 → 编排」中创建名为 `tnr-atlas` 的编排（不同 1Panel 版本的菜单文字可能略有差异）。粘贴以下 Compose，再设置下一节的环境变量。

没有 `build:` 字段，服务器只拉取镜像。端口默认仅绑定服务器的 `127.0.0.1`，通过 1Panel 反代对外提供访问。

<!-- COMPOSE_START -->
```yaml
name: tnr-atlas
services:
  tnr:
    image: ${TNR_IMAGE:?请设置 GitHub Actions 发布的 GHCR 镜像地址}
    restart: unless-stopped
    ports:
      - "127.0.0.1:${TNR_PORT:-8081}:8081"
    environment:
      PUBLIC_ORIGIN: ${TNR_PUBLIC_ORIGIN:?请设置 TNR 公开访问地址}
      TNR_ACCESS_CODE: ${TNR_ACCESS_CODE:?请设置初始公共访问码}
      TNR_ADMIN_PASSWORD: ${TNR_ADMIN_PASSWORD:?请设置独立的管理员密码}
      ONEIMG_BASE_URL: http://oneimg:8080
      ONEIMG_TOKEN: ${ONEIMG_TOKEN:-}
      ONEIMG_FOLDER_ID: ${ONEIMG_FOLDER_ID:-0}
    volumes:
      - tnr-data:/app/data
    depends_on:
      - oneimg
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    read_only: true
    tmpfs:
      - /tmp:size=256m,mode=1777
    stop_grace_period: 30s
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

  oneimg:
    image: ${ONEIMG_IMAGE:-onexru/oneimg-oneimg:latest}
    restart: unless-stopped
    ports:
      - "127.0.0.1:${ONEIMG_PORT:-8082}:8080"
    environment:
      SERVER_PORT: "8080"
      APP_URL: ${ONEIMG_PUBLIC_ORIGIN:?请设置图床公开访问地址}
      DB_TYPE: sqlite
      SQLITE_PATH: /app/data/data.db
      UPLOAD_ROOT: /app/uploads
      DEFAULT_USER: admin
      DEFAULT_PASS: ${ONEIMG_INITIAL_PASSWORD:?请设置图床初始管理员密码}
      MAX_FILE_SIZE: "10485760"
      ALLOWED_TYPES: image/jpeg,image/png,image/webp
      TRUSTED_PROXIES: ${ONEIMG_TRUSTED_PROXIES:-}
    volumes:
      - oneimg-data:/app/data
      - oneimg-uploads:/app/uploads
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  tnr-data:
  oneimg-data:
  oneimg-uploads:
```
<!-- COMPOSE_END -->

### 环境变量

将根目录 `.env.example` 的内容填入 1Panel 的编排环境配置，或复制为该编排目录中的 `.env` 文件。不要提交填好密码的 `.env`。

| 变量 | 示例 / 要求 |
| --- | --- |
| `TNR_IMAGE` | `ghcr.io/alumos/tnr-atlas:latest`；正式运行建议使用验收后的 `sha-...` 固定标签 |
| `TNR_PUBLIC_ORIGIN` | `https://cats.example.com`，不带末尾 `/` 和额外路径 |
| `ONEIMG_PUBLIC_ORIGIN` | `https://img.example.com`，浏览器必须能访问 |
| `TNR_ACCESS_CODE` | 初始公共访问码，16–72 字节，仅用于空数据库初始化 |
| `TNR_ADMIN_PASSWORD` | 独立管理员密码，至少 16 字节，与公共访问码不同 |
| `ONEIMG_INITIAL_PASSWORD` | oneimg 首次创建管理员时使用的独立强密码 |
| `ONEIMG_TOKEN` | 首次可留空；完成图床初始化后填写 |
| `ONEIMG_FOLDER_ID` | 首次为 `0`；建立 `NTR` 文件夹后填该文件夹数字 ID |
| `TNR_PORT` | 默认 `8081`，服务器回环端口 |
| `ONEIMG_PORT` | 默认 `8082`，服务器回环端口 |
| `ONEIMG_IMAGE` | 默认官方 `onexru/oneimg-oneimg:latest`；验收后建议固定实际官方版本或 digest |
| `ONEIMG_TRUSTED_PROXIES` | 实际反向代理的来源 IP/CIDR；按服务器部署形态填写 |

`TNR_ACCESS_CODE`、`TNR_ADMIN_PASSWORD`、图床管理员密码应分别设置，不要使用示例标记或相同密码。普通 ASCII 随机码便于控制字节数。

如果使用命令行管理同一个编排，可执行：

```bash
docker compose config --quiet
docker compose pull
docker compose up -d
docker compose ps
```

首轮部署时图床 Token 为空是正常的：平台可浏览空档案库和进入后台，但会提示图床尚未配置，阻止上传。

### 持久化

Compose 创建三个命名数据卷：

| 卷 | 内容 |
| --- | --- |
| `tnr-data` | TNR SQLite、志愿者访问码哈希、会话、审计历史 |
| `oneimg-data` | oneimg 数据库、配置和自动持久化的密钥 |
| `oneimg-uploads` | 图床图片文件 |

卷的实际名称通常带编排项目前缀。升级和重新创建容器不会删除命名卷。不要删除卷或使用 `down -v`，否则会丢失数据。

TNR 容器以 UID/GID `10001:10001` 运行，命名卷首次创建会继承镜像中 `/app/data` 的所有权。若自行改成服务器绑定目录，需在服务器上确保该目录允许此 UID 写入；不要对本机 Mac 安装目录做权限调整。

## 4. 配置 1Panel 网站反向代理

创建两个反向代理站点并启用 HTTPS：

| 域名 | 代理目标 |
| --- | --- |
| `cats.example.com` | `http://127.0.0.1:8081` |
| `img.example.com` | `http://127.0.0.1:8082` |

这适用于反向代理可以访问服务器回环端口的部署形态。如果 OpenResty 运行在隔离的容器网络内，`127.0.0.1` 指向 OpenResty 自己，此时应把现有反代和本编排服务加入同一个 Docker 网络，并分别代理 `http://tnr:8081` 与 `http://oneimg:8080`。

这只是加入 **1Panel 服务器现有网络**，仍然只有两个应用容器。可在 Compose 两个服务下增加 `networks: [default, panel]`，并在文件末尾增加：

```yaml
networks:
  panel:
    external: true
    name: 你的现有OpenResty网络名
```

不要照抄一个不确定的网络名称。只在部署服务器的 1Panel 中确认已有反代网络，不涉及开发 Mac 的网络配置。

TNR 站点的反代建议设置：

```nginx
client_max_body_size 52m;
proxy_read_timeout 300s;
proxy_send_timeout 300s;
proxy_set_header Host $host;
proxy_set_header X-Forwarded-Proto $scheme;
```

oneimg 站点可设置 `client_max_body_size 12m;`。上面的设置应放在相应站点或代理位置中，避免与 1Panel 已生成的重复指令冲突。

`ONEIMG_TRUSTED_PROXIES` 应匹配实际反代出口，避免 oneimg 将所有用户计入同一个登录限流预算。官方支持本机反代 `127.0.0.1` 或指定 Docker 子网等配置，具体取值以服务器实际来源为准。不要无条件信任全部公网地址。

## 5. 初始化官方图床

1. 打开 `https://img.example.com`，使用 `admin` 和你设置的 `ONEIMG_INITIAL_PASSWORD` 登录。该环境变量仅在首次创建管理员时生效，不能用于重置已存在账号的密码。
2. 推荐在 oneimg 内创建一个 **TNR 专用普通用户**，后续目录和 Token 都属于这个用户，避免和个人图库混用。
3. 用这个用户进入图库，创建名为 **`NTR`** 的文件夹。目录名按照本项目约定使用 `NTR`；你也可以手工取其他名字，TNR 只使用数字 ID。
4. 打开该文件夹，在页面 URL 中找到 `folder_id=数字`，把数字填为 `ONEIMG_FOLDER_ID`。目录必须和生成 Token 的用户属于同一账号。
5. 在图床的系统上传设置中打开 **「保存源文件名」**。这样 `NTR-猫咪名-相册编号-照片编号` 才会按原前缀保留。源文件是 `.jpg/.png/.webp`，最终扩展名由 oneimg 根据实际编码生成。若使用自定义名称模板，也应保留 `{filename}`。
6. 在 oneimg 打开 **「图片压缩」（`compress_image`）、「保存 WebP」（`save_webp`）和「缩略图」（`thumbnail`）**，保存设置。全部图片处理由官方图床执行，TNR 不自行转码，也不上传第二份缩略图。当前核验源码的缩略图为 300×300 内、质量 80，主图保持图床策略，不承诺固定尺寸。单文件上限保持 10 MiB。若图床响应未提供 WebP 主图和独立缩略图，TNR 会报错，已上传文件保留供后台核对。
7. 开启 oneimg 的 API 访问能力，按图床账号界面生成上传凭据，将 Token 填入 `ONEIMG_TOKEN`。不要把浏览器 Cookie、管理员密码或其他账户的 Token 填入这里。
8. 重新创建 **`tnr` 服务**使新环境变量生效；不需要重建镜像：

```bash
docker compose up -d --no-deps tnr
```

若暂时不设置目录，`ONEIMG_FOLDER_ID=0` 会上传到该用户的未分类区域，唯一文件名仍然有效。设置了非零目录后，TNR 会核验图床返回的 `folder_id`，避免接口版本不兼容时悄悄放错目录。

### 关于目录和删除

这里使用的是 **一个单层 `NTR` 文件夹**，不是 `NTR/小橘/` 这样的嵌套目录。官方 Token 无法自动建目录或新标签；本项目不会通过登录 Cookie 或修改数据库绕过限制。

每只猫的照片靠固定相册编号和名字前缀区分，同名猫咪也有不同编号。猫咪在平台改名后相册编号不变，原文件无需移动或重命名。

删除 TNR 猫咪记录、移除相册图片、或遇到部分上传失败，**图床文件保留**。需要清理时，由你在官方图床后台人工核对与处理。本项目不会调用图床删除接口。

### Token 到期

核验的官方源码中，API Token 默认有效期是 **30 天**。请以所部署版本的图床界面显示为准，安排在到期前更新凭据。新 Token 写入 1Panel 的 `ONEIMG_TOKEN` 后重新创建 `tnr` 服务。

Token 到期时已保存照片仍可访问，但新增上传可能返回 401。TNR 不储存图床用户密码，也不会自动绕过图床登录验证。

## 6. 配置 TNR 管理后台

访问 `https://cats.example.com/admin`，输入 `TNR_ADMIN_PASSWORD`。

### 公共访问码

「录入访问设置」可以启用或停用公共入口，也可以设置新公共访问码。新码以 bcrypt 哈希保存在 TNR 数据卷中：

- 重启、拉取新镜像或重建容器后仍有效。
- 修改 `.env` 中的 `TNR_ACCESS_CODE` 不会覆盖后台已保存的新码；该变量仅用于空库初始化。
- 保存公共访问设置会撤销旧公共入口会话。独立志愿者账号不受影响。
- 原码不会回显；忘记时直接设置新码并通过团队内部渠道告知志愿者。

### 独立志愿者

点击「新增志愿者」，输入唯一登录账号、公开昵称、个人访问码和账号状态。

- 账号使用 3–32 位字母、数字、下划线或连字符。
- 昵称 1–40 字，会公开显示为上传人，建议使用志愿者昵称而非真实个人信息。
- 个人访问码为 16–72 字节，和公共码分开。
- 志愿者登录后，服务端自动将上传人固定为该账号的昵称。
- 停用、删除、修改账号或重置个人码后，原登录立即失效；历史猫咪档案和审计仍保留。

### 猫咪资料和相册

后台可编辑姓名、性别、毛色、绝育状态、描述与公开上传昵称。点击照片可设为封面，至少保留一张照片，也可以追加到最多六张。

添加照片会先保存当前修改，再执行上传。若追加上传失败，之前保存的文字修改仍然有效，界面会显示错误；已上传但未关联的文件可能在图床保留。

删除猫咪资料有确认弹窗；猫咪从公开页面移除，图床文件保留。审计记录保存删除前快照与图片地址。

### 审计

可查看成功登录、公共入口设置、志愿者增删改、猫咪创建/修改/删除、照片追加、已知未完成上传的保留记录。修改/删除记录可点「查看」读取快照。密码、访问码和图床 Token 不写入审计。

当前仅有一个部署管理员身份，没有多管理员账号或失败登录事件的完整审计链。服务器层日志和访问限流可由现有 1Panel 配置补充。

## 7. 首次验收

首次使用你部署的真实官方图床时，按以下流程验收：

1. 检查 `docker compose ps`：只有 `tnr`、`oneimg` 两个应用服务，TNR 状态健康。
2. 打开 TNR，初始列表应为空；`/?demo=1` 是明确标注的演示，不是已录入数据。
3. 在后台创建一个测试志愿者，用独立账号录入一只测试猫，填写全部必填字段并上传两张照片。
4. 用普通 JPEG/PNG 上传两张照片，在 oneimg 的 `NTR` 文件夹内应出现 **两条图片记录**，每条由图床生成 WebP 主图和缩略图，文件名带同一猫咪相册编号。检查真实输出格式与缩略图尺寸；当前核验源码的缩略图限制为 300×300 内，主图没有固定 1600px 约定。
5. 在 TNR 检查缩略图、详情大图、筛选、手机布局和主题。图片 URL 应以你的图床公开域名开头，不应为 `http://oneimg:8080`。
6. 修改绝育状态和名字，确认统计/搜索更新，相册仍然正确关联。
7. 更换公共码、停用测试志愿者，确认旧入口失效。查看相应审计。
8. 删除测试猫，确认 TNR 不再显示；图床照片仍在；审计可以查看原档案快照。

本地开发已验证的是官方接口契约模拟和浏览器流程。实际官方镜像、你的域名、证书及图床设置仍需完成上述服务器验收，不能用本地模拟替代。

## 8. 更新、回滚与备份

### 更新

在 GitHub Actions 构建通过后，在 1Panel 将 `TNR_IMAGE` 改为新的固定提交标签，重新拉取并创建 `tnr`。保留全部数据卷。

```bash
docker compose pull tnr
docker compose up -d --no-deps tnr
```

图床独立升级官方镜像；升级前阅读上游发布说明，升级后重新执行上传验收。不要因为 TNR 更新而自动把图床切到另一个未经验收的版本。

### 回滚

把 `TNR_IMAGE` 改回之前验证过的提交标签并重新创建 TNR。若未来版本涉及数据库迁移，应先核对对应版本是否支持回退；本初始版本没有已发布的历史迁移。

### 一致性备份

需要同时备份 TNR 数据、oneimg 数据和上传文件。oneimg 的数据目录还包含加密/会话密钥，不能只备份图片。

最直接的一致性方式是在维护时段停两个服务后复制数据，复制完成再启动，不需要额外部署备份容器：

```bash
backup_dir="./backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$backup_dir"
docker compose stop tnr oneimg
docker compose cp -a tnr:/app/data "$backup_dir/tnr-data"
docker compose cp -a oneimg:/app/data "$backup_dir/oneimg-data"
docker compose cp -a oneimg:/app/uploads "$backup_dir/oneimg-uploads"
docker compose start oneimg tnr
```

复制失败时先保留现场，修复备份路径或空间问题后再确认服务启动。`.env` 或 1Panel 编排环境配置应另行加密备份，和公开代码分开。

恢复时先停服并备份现有卷，使用新的空数据卷恢复完整目录，保留原 UID/GID，TNR 目录属主为 `10001:10001`。恢复 TNR 数据库时也恢复对应时间点的图床数据库、密钥和照片；不要把正在运行的 SQLite 数据库与旧 WAL 文件混合覆盖。确认数据和权限后再启动并验收。

## 9. 常见问题

| 现象 | 排查 |
| --- | --- |
| 站点 502 | 1Panel 反代目标是否处于同一网络；不要把反代容器自己的 127.0.0.1 当成宿主机 |
| 新图片无法上传，旧图正常 | 查看 Token 是否过期、API 是否开启、用户配额是否足够 |
| 提示目录无效 | `folder_id` 是否存在，是否属于 Token 对应用户；官方镜像是否包含已核验的目录功能 |
| 提示来源无效 | `TNR_PUBLIC_ORIGIN` 是否和浏览器地址完全一致，是否错误加了末尾 `/`，HTTP/HTTPS 是否一致 |
| HTTPS 下反复要求登录 | 公开地址、HTTPS 和代理配置是否正确；开发 HTTP 不应使用 HTTPS 的公开地址配置 |
| 图片地址是 Docker 服务名 | oneimg 的 `APP_URL` 应为 `https://img.example.com`，不是容器内地址 |
| 大图返回 413 | TNR 反代 52 MiB 请求上限、单张 10 MiB、相册 50 MiB 三者同时满足；像素还须不超过 2400 万 |
| 手机 HEIC 无法选择 | 先在设备上导出 JPEG；当前支持 JPEG/PNG/WebP |
| 文件名没有猫咪前缀 | 打开 oneimg「保存源文件名」，或保留 `{filename}` 模板 |
| 提示图床未返回 WebP 主图或独立缩略图 | 在 oneimg 开启「图片压缩」「保存 WebP」「缩略图」并保存；检查版本和存储是否支持缩略图；失败时已上传文件会保留 |
| 删除猫咪后图床照片还在 | 这是使用官方原版的预期行为，TNR 不自动删图 |
| 修改初始公共码环境变量无效 | 已初始化后请到 `/admin` 设置；环境变量不会覆盖持久化设置 |
| TNR 数据卷写入失败 | 绑定目录权限是否允许 UID 10001；默认命名卷无需额外处理 |

完整设计和 API 见 [ARCHITECTURE.md](ARCHITECTURE.md)。
