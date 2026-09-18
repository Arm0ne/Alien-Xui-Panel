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

## 入站限速

在「入站 → 编辑」里填 **入站限速（Mbps）** 即可限制整个入站端口的下载速度（该入站下所有用户共享此额度，`0` 表示不限速）。

限速由内核的 tc（HTB）完成——Xray-core 本身不提供任何按用户限速能力，面板也不再把限速值伪装成 Xray 的 policy level。原理、验证命令与故障排查见 **[docs/SPEED_LIMIT.md](docs/SPEED_LIMIT.md)**。
