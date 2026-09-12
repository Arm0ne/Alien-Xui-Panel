# Alien-Panel

个人使用的 Xray 面板程序，默认按 Linux 原生方式部署。

## 本地构建

```text
go test ./...
go build ./...
```

Linux 安装和面板管理使用项目根目录的 `install.sh` 与 `x-ui.sh`。运行时数据默认保存在 `/etc/x-ui`，程序目录默认是 `/usr/local/x-ui`。

项目保留 `x-ui` 命令、服务名和数据路径，用于兼容现有安装与数据迁移。
