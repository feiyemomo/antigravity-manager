# Google Antigravity Multi-Account Tool (Go + Gin Native)

高可用、高性能的 Google Antigravity (AGY) 多账户无缝原子切换与 API 代理工具。基于 **Go 1.26 + Gin** 原生重构，提供单文件免环境运行、Windows 原生凭据管理器对接、IDE 实时热同步（Hot-Sync）以及与 Antigravity 原生风格一致的额度仪表盘。

---

## 核心架构设计

```text
agy-tools/
├── cmd/
│   └── agy-tools/
│       └── main.go              # Cobra CLI 命令行接口 (start, doctor, switch, accounts, profile, rotate)
├── pkg/
│   ├── config/                  # 路径常量与配置规范 (~/.agy_auth, ~/.agy-tools, ~/.gemini)
│   ├── types/                   # 领域模型定义 (Account, QuotaData, ProfilePayload, DoctorReport)
│   ├── core/
│   │   ├── credential/          # Windows 原生 Credential Manager (advapi32.dll CredReadW/CredWriteW)
│   │   ├── process/             # 进程安全检测与语言服务器重启 (CreateToolhelp32Snapshot)
│   │   ├── symlink/             # NTFS 目录联接与共享目录管理 (conversations, skills)
│   │   ├── profile/             # 隔离 Profile 读写 (~/.agy_auth/profiles/<name>/)
│   │   ├── livesync/            # Chrome DevTools Protocol WebSocket 实时内存热同步
│   │   ├── switcher/            # 快照回滚保证的事务原子切换引擎
│   │   └── doctor/              # 系统健康自检与自愈模块 (--fix)
│   ├── auth/                    # Google OAuth 2.0 PKCE 流程与自动 Project/Tier 解析
│   ├── quota/                   # Antigravity CloudCode API 额度获取与分组解析
│   ├── store/                   # 线程安全账户持久化存储与定时刷新
│   ├── rotator/                 # 后台定时自动轮换服务
│   └── server/
│       ├── server.go            # Gin Web 引擎构建与中间件配置
│       ├── handlers/            # REST API 控制器 (账户管理、切换、OAuth 回调、配置)
│       ├── proxy/               # OpenAI 兼容模型路由 (/v1/models)
│       └── web/
│           ├── embed.go         # 静态前端资源嵌入 (Go embed.FS)
│           └── index.html       # Antigravity 原生暗色风格仪表盘 (SVG 进度环与一键切换)
```

---

## 核心亮点

1. **单文件原生二进制 (Native Windows Binary)**
   - 彻底摒弃臃肿的 Node SEA blob，纯 Go 编译，体积仅 ~30MB，秒级启动，极低内存消耗。
2. **Windows 原生安全凭据存储**
   - 优先通过 `advapi32.dll` 的 `CredReadW` / `CredWriteW` / `CredDeleteW` 直接操作 Windows Credential Manager 中的 `gemini:antigravity` 目标，防止凭据泄露。
3. **IDE 在线无缝热同步 (Live Hot-Sync)**
   - 通过 Chrome DevTools Protocol (CDP) WebSocket 直接向正在运行的 Antigravity IDE 注入新账号凭据并触发 `cloudCodeService.retrieveUserQuotaSummary`，无需频繁退出或重启 IDE 即可实现额度和账号实时生效。
4. **全隔离 Profile 与符号链接共享**
   - 严格隔离敏感认证文件（`installation_id`, `antigravity-oauth-token`, `settings.json`）。
   - 共享数据（`conversations/`, `skills/`）通过 NTFS 目录联接自动重定向至 `~/.agy_auth/shared/`，切换账号无需重新配置自定义技能与聊天记录。
5. **Antigravity 原生风格额度看板与中英双语**
   - 完整复刻 Antigravity 原生额度卡片（Gemini Models / Claude and GPT models 分组），包含 Weekly 与 5-Hour 限制的实时百分比、SVG 圆环进度条以及精确倒计时。
   - 支持 **🌐 中英双语 (Chinese / English)** 一键无刷新切换，本地自动持久化语言偏好。
6. **暗色 / 亮色双主题系统 (Dark & Light Theme)**
   - 包含深邃极客暗色模式与现代清爽白底亮色模式，纯 CSS 变量驱动，配额数值与界面元素 0 延迟平滑变色。
7. **⚡ 自动换号并继续 (额度耗尽断点接力)**
   - 顺位切换下一账号 -> 语言服务原子重启生效新配额 -> 自动切回活跃会话 -> 模拟 DOM 点击与操作系统级原生 Enter 自动直接发送“继续”，支持全自动检测额度耗尽自动切号开关。
8. **一键双击无黑框单文件运行 (GUI Subsystem)**
   - 打包为原生 Windows GUI 单文件程序 `Antigravity-Manager.exe`，完全不弹黑色控制台窗口，内置单实例检测防重冲突，双击自动秒开浏览器控制台。

---

## 快速安装与编译

### 编译单文件 GUI 版本 (无黑框、双击秒开)

```bash
# 编译单文件绿色 GUI 版 (推荐日常使用)
go build -ldflags="-H windowsgui -s -w" -o Antigravity-Manager.exe ./cmd/agy-tools

# 编译命令行版本
go build -ldflags="-s -w" -o agy-tools.exe ./cmd/agy-tools
```

---

## 常用命令指南

### 1. 启动 Web 仪表盘与服务
```bash
# 启动后台服务与仪表盘 (默认端口 38080)
agy-tools start

# 指定端口并禁止自动弹出浏览器
agy-tools start --port 38080 --no-open
```
访问 `http://127.0.0.1:38080` 即可进入图形化额度与账号管理看板。

### 2. 添加 Google 账号
```bash
agy-tools login
# 或
agy-tools accounts add
```
会自动打开浏览器进行 Google OAuth2 授权，并在完成后将凭据安全录入。

### 3. 查看账号与实时额度
```bash
agy-tools accounts list
```
输出样例：
```text
Authenticated Google Accounts:
--------------------------------------------------------------------------------
[*] feiyemomo@gmail.com (mo momo, Tier: g1-pro-tier)
    ID   : e98db96a-feb3-4273-a552-d6f216e1f1e9
    Quota: [Gemini Models: Weekly Limit Remaining: 99% (reset 2026-10-07T01:05:07Z), Five Hour Limit Remaining: 94% (reset 2026-09-30T06:05:07Z)] [Claude and GPT models: Weekly Limit Remaining: 100% (reset 2026-10-07T02:22:05Z), Five Hour Limit Remaining: 100% (reset 2026-09-30T07:22:05Z)]

[ ] momo324423@gmail.com (mo momo, Tier: g1-pro-tier)
    ID   : 24f4160d-9c52-41cc-aa5e-2922efe30396
    Quota: [Gemini Models: Weekly Limit Remaining: 57% (reset 2026-10-02T12:07:31Z), Five Hour Limit Remaining: 30% (reset 2026-09-30T05:21:02Z)] [Claude and GPT models: Weekly Limit Remaining: 67% (reset 2026-10-03T13:36:59Z), Five Hour Limit Remaining: 100% (reset 2026-09-30T07:21:52Z)]
--------------------------------------------------------------------------------
```

### 4. 账号原子切换
```bash
# 切换至指定账号或 Profile (在 IDE 运行时自动进行热同步)
agy-tools switch feiyemomo@gmail.com --live

# 轮换至下一个可用账号
agy-tools rotate
```

### 5. 系统健康自检与自愈
```bash
# 执行全面自检（检查进程状态、凭据管理器、Profile 与符号链接）
agy-tools doctor

# 自动修复缺失的目录与符号链接
agy-tools doctor --fix
```

---

## 协议与许可

MIT License
