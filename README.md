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

## 订阅默认设置

- 订阅服务**默认开启**，监听端口 `58888`；
- 订阅路径默认 `/yfzgsub/`，即 `http://IP:58888/yfzgsub/<subId>`；
- JSON 订阅路径默认 `/yfzgjson/`，即 `http://IP:58888/yfzgjson/<subId>`；
- 升级已有面板时，如果这些设置仍是旧默认值（`false` / `13788` / `/sub/` / `/json/`），面板会在首次启动时自动升级为新默认值，并在日志里记录；**用户自己改过的值不会被覆盖**。
- 订阅证书/私钥留空时，自动沿用**面板证书**（订阅设置页会直接显示面板的证书路径），所以配好面板证书后订阅链接自动就是 `https://`。
- 订阅服务启动失败（例如 58888 端口被占用）**只记录告警，不影响面板启动**。

> 两条路径不能完全相同：订阅与 JSON 订阅注册的都是 `GET :subid` 路由，路径重复会让 Gin 直接 panic、面板无法启动，所以 JSON 订阅用同级前缀 `/yfzgjson/`。
>
> 另外记得在防火墙/安全组放行 TCP `58888`，否则客户端拿不到订阅。
