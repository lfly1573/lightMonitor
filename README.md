# lightMonitor

lightMonitor 是一个基于 Go、Gin、SQLite 和 Vue3 的轻量级监控系统。

## 运行目录

- `data`: SQLite 数据文件
- `log`: 日志文件

## 本地启动

```bash
go run ./cmd/lightmonitor -P 8573
```

首次启动时访问 `/install` 或任意前端页面完成管理员初始化。后端安装状态接口为 `/api/install/status`。
