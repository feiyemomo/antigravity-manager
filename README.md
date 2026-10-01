# Antigravity Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/feiyemomo/antigravity-manager)](https://github.com/feiyemomo/antigravity-manager/releases)
[![Platform](https://img.shields.io/badge/platform-Windows%20x64-lightgrey)](https://github.com/feiyemomo/antigravity-manager)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](go.mod)

高可用、高性能的 Google Antigravity (AGY) 多账户无缝原子切换、实时配额监控与自动化断点接力管理器。基于 **Go + Gin** 原生重构，提供单文件绿色免安装、无黑框后台运行、Windows 原生凭据管理器安全存储、IDE 实时热同步（Hot-Sync）以及与 Antigravity 原生视觉风格高度一致的现代化中英双语 Web 仪表盘。

---

## 核心亮点

1. **单文件原生绿色版 (Zero-Dependency Single Binary)**
   - 彻底摒弃臃肿环境与外部依赖，纯 Go 静态编译，原生 Windows GUI 模式，启动无黑色控制台闪烁，内存消耗极低。
2. **Windows 原生安全凭据集成**
   - 深度集成 Windows 原生凭据管理器（Credential Manager，`advapi32.dll`），严密隔离并安全保管 Google OAuth 凭据，杜绝明文风险。
3. **IDE 内存级无缝热同步 (Live Hot-Sync)**
   - 通过 Chrome DevTools Protocol (CDP) WebSocket 直接向正在运行中的 Antigravity IDE 注入新账号凭据并触发配额刷新，无需退出或重启 IDE 即可实现额度和账号实时生效。
4. **多配置文件完全隔离与资产共用**
   - 账户敏感认证数据（`installation_id`, `antigravity-oauth-token`, `settings.json`）完全隔离。
   - 资产数据（`conversations/`, `skills/` 等）基于 NTFS 目录联接安全映射共享，换号不丢历史会话与自定义扩展。
5. **⚡ 智能换号断点接力 (Auto Switch & Continue)**
   - 支持全自动额度监控与断点接力：额度耗尽自动切换下一个健康账号 -> 语言服务器原子生效 -> 自动切回活跃会话 -> 模拟 DOM 并触发原生回车直接发送“继续”，全程无需人工干预。
   - 提供可视化开关控制与配置防误触。
6. **Antigravity 原生视效看板与中英双语 (Bilingual Dashboard)**
   - 1:1 原生级深邃暗色/清爽亮色主题双模支持，纯 CSS 变量驱动，配额数值与圆形进度环 0 延迟秒级变色。
   - 内置 **🌐 中英双语 (Chinese / English)** 一键无刷新平滑切换，自动持久化存储语言偏好。

---

## 架构概览

```text
antigravity-manager/
├── cmd/
│   └── agy-tools/
│       └── main.go              # Cobra CLI 与 GUI 入口 (start, doctor, switch, accounts, rotate)
├── pkg/
│   ├── config/                  # 路径常量与存储配置 (~/.agy_auth, ~/.gemini)
│   ├── types/                   # 核心领域模型 (Account, QuotaData, ProfilePayload, DoctorReport)
│   ├── core/
│   │   ├── credential/          # Windows 原生 Credential Manager 对接
│   │   ├── process/             # 进程检测与语言服务平滑重启
│   │   ├── symlink/             # NTFS 目录联接与共享资产管理
│   │   ├── profile/             # Profile 隔离读写引擎
│   │   ├── livesync/            # Chrome DevTools Protocol 实时热同步
│   │   ├── switcher/            # 事务性原子切换引擎
│   │   └── doctor/              # 系统环境健康诊断与自愈修复
│   ├── auth/                    # Google OAuth 2.0 PKCE 流程与自动 Project/Tier 解析
│   ├── quota/                   # Antigravity CloudCode API 额度获取与分组解析
│   ├── store/                   # 线程安全账户持久化存储与定时刷新
│   ├── rotator/                 # 智能轮换与自动化断点接力引擎
│   └── server/
│       ├── server.go            # Gin Web 引擎构建
│       ├── handlers/            # REST API 控制器 (账号管理/切换/回调/开关)
│       └── web/
│           ├── embed.go         # 静态前端资源嵌入 (Go embed.FS)
│           └── index.html       # 原生双模双语前端控制面板
```

---

## 下载与使用

### 1. 下载预编译发行版
前往 [GitHub Releases](https://github.com/feiyemomo/antigravity-manager/releases) 下载最新发行版压缩包：
- `Antigravity-Manager-v2.0.0-windows-amd64.zip`

解压后包含：
- `Antigravity-Manager.exe`: **直接双击运行即可**。后台无黑框驻留并自动唤起浏览器打开管理面板。
- `agy-tools.exe`: 命令行 CLI 工具，适合脚本调用或终端极客使用。
- `start-manager.bat` / `stop-manager.bat`: 便捷启动与停止控制脚本。

### 2. 源码本地编译

确保系统已安装 Go 1.22 或更高版本：

```bash
# 克隆仓库
git clone https://github.com/feiyemomo/antigravity-manager.git
cd antigravity-manager

# 编译无黑框 GUI 单文件版 (推荐日常使用)
go build -ldflags="-H windowsgui -s -w" -o Antigravity-Manager.exe ./cmd/agy-tools

# 编译命令行 CLI 版本
go build -ldflags="-s -w" -o agy-tools.exe ./cmd/agy-tools
```

---

## 常用 CLI 命令指南

### 启动服务
```bash
# 启动后台服务与仪表盘 (默认端口 38080)
agy-tools start

# 指定端口并禁止自动唤起浏览器
agy-tools start --port 38080 --no-open
```

### 账号管理
```bash
# 登录并授权新的 Google 账号
agy-tools login

# 查看已授权的所有账号及其剩余配额
agy-tools accounts list
```

### 账号切换与健康检查
```bash
# 切换至指定账号并触发 IDE 在线热同步
agy-tools switch user@gmail.com --live

# 顺位轮换至下一个配额健康账号
agy-tools rotate

# 执行系统健康检查与自动修复
agy-tools doctor --fix
```

---

## 开源协议

本项目采用 [MIT License](LICENSE) 开源许可证。
Copyright (c) 2026 feiyemomo
