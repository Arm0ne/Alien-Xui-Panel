# Alien-Panel

个人使用的 Xray 面板程序，默认按 Linux 原生方式部署。

## 安装

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Arm0ne/Alien-Xui-Panel/main/install.sh)
```

已安装的机器上执行 `x-ui`，选菜单第 2 项即可升级，数据不会丢失。

> 目前只提供 linux/amd64（x86_64）资产，ARM 机器暂不支持。

## 发布新版本

改完代码后打一个 `v` 开头的标签并推送，GitHub Actions 会自动编译、打包并发布：

```bash
git tag v1.0.3 && git push origin main --tags
```

完整的发版流程、Xray 内核升级方法、故障排查等，见 **[docs/RELEASE.md](docs/RELEASE.md)**。


Linux 安装和面板管理使用项目根目录的 `install.sh` 与 `x-ui.sh`。运行时数据默认保存在 `/etc/x-ui`，程序目录默认是 `/usr/local/x-ui`。

项目保留 `x-ui` 命令、服务名和数据路径，用于兼容现有安装与数据迁移。
