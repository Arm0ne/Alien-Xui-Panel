# Alien-Panel 发布与运维手册

面向个人使用的操作手册：从改代码到多台 VPS 部署的完整流程。

---

## 速查：最常做的三件事

| 我想…… | 怎么做 |
|---|---|
| **发一个新版本** | `git tag v1.0.3 && git push origin main --tags` |
| **给新 VPS 装面板** | `bash <(curl -Ls https://raw.githubusercontent.com/Arm0ne/Alien-Xui-Panel/main/install.sh)` |
| **升级已有面板** | 服务器上执行 `x-ui`，选菜单第 **2** 项 |

发版全程约 2.5 分钟，不需要本地编译，也不需要手动上传任何文件。

---

## 一、日常发版：三步

```bash
# 1. 提交代码
git add -A
git commit -m "改了什么"

# 2. 打标签（版本号格式见下节）
git tag v1.0.3

# 3. 推送代码和标签
git push origin main --tags
```

推送标签后 GitHub Actions 自动接管：编译 → 打包 → 创建 Release → 上传资产 → 标记为 Latest。
你可以直接关电脑，回来后看 Actions 页面确认即可。

> **标签就是版本号**，不需要手动修改 `config/version`。
> 工作流会在编译前把标签号（去掉 `v`）写进该文件，再 `//go:embed` 嵌进二进制。

---

## 二、版本号规则

**必须是小写 `v` + 三段纯数字**，例如 `v1.0.3`。

- 合规：`v1.0.3`、`v2.0.0`、`v1.12.5`
- 不合规：`v1.0`（只有两段）、`V1.0.3`（大写）、`v1.0.3-beta`（带后缀）、`release-1.0.3`

格式不对时工作流会在第一步直接失败并给出提示，不会产出错误的包。

**为什么必须小写 `v`：** `x-ui.sh` 的「自定义版本」功能会给用户输入自动补一个小写 `v`，
若标签是大写 `V`，该功能会拼出 `v1.0.3` 而下载链接 404。
另外 `install.sh` 打印的是 `v${xui_version}`，所以 `config/version` 里也不能带 `v`，
否则会显示成 `vV1.0.3`。

**建议的递增方式：**

| 改动性质 | 例子 |
|---|---|
| 修 bug、调文案、改界面 | `1.0.2` → `1.0.3` |
| 加功能 | `1.0.3` → `1.1.0` |
| 大改、不兼容变更 | `1.1.0` → `2.0.0` |

**升版本号还有一个隐藏作用**：面板所有静态资源（JS/CSS）都带
`?{{ .cur_ver }}` 查询串，且缓存头是 `max-age=31536000`（一年）。
升版本号等于缓存自动失效，用户的浏览器才会重新拉取改动过的前端文件。

---

## 三、CI 自动做了什么

| 步骤 | 说明 |
|---|---|
| 解析标签 | 校验必须是 `vX.Y.Z` 格式 |
| 写入版本号 | 标签号写入 `config/version` |
| 安装 Zig | 交叉编译用的 C 工具链（约 10 秒） |
| 编译 | `CGO_ENABLED=1` + Zig，产出 linux/amd64 面板二进制 |
| 二进制体检 | 见下方「五道防线」 |
| 准备 bin/ 目录 | 复用上一版的 geo 数据 + 下载固定版 Xray 内核 |
| 组装打包 | 打包成 `x-ui-linux-amd64.tar.gz` |
| 包体检 | 解包复核结构、权限、内层二进制哈希 |
| 创建 Release | 上传资产并标记为 Latest |

### 五道防线（任一不过就不发布）

1. **cgo 桩检查** —— 若产物是 `CGO_ENABLED=0` 编出的 sqlite 桩实现（能编译但一启动就崩），直接失败
2. **ELF 架构检查** —— 确认是 x86-64 可执行文件
3. **版本号检查** —— 确认新版本号真的嵌进了二进制
4. **glibc 基线检查** —— 确认最高只需 glibc 2.28，否则老 VPS 装不上
5. **内核版本检查** —— 除 SHA256 校验外，还会实跑 `xray -version` 确认自报版本

---

## 四、Xray 内核：固定版本

本面板对 Xray 内核采取**锁死**策略，因为新版本不一定比旧版本稳定。
CI 每次都下载指定的那个版本，**绝不会跟着上游自动升级**。

### 当前锁定

| 项目 | 值 |
|---|---|
| 版本 | `26.6.1` |
| 官方 zip 的 SHA256 | `136e822e99e616692550723e8b607cd8858c62a390aea5704938bc27930904ba` |
| 内核二进制 SHA256 | `402c34d5d537a48c858d6d227c1e660eda8bf8abe9d1258d54faa66b8b263b78` |

### 想升级内核时

改 `.github/workflows/release.yml` 顶部 `env:` 段里的两个值：

```yaml
XRAY_VERSION: '26.6.1'                    # 改成新版本号，如 26.9.9
XRAY_ZIP_SHA256: '136e822e...904ba'       # 换成对应的校验值
```

**去哪拿这两个值：**

1. 打开 https://github.com/XTLS/Xray-core/releases ，找到目标版本（例如 `v26.9.9`）
2. 下载 `Xray-linux-64.zip.dgst` 这个几十字节的小文件，
   其中 `SHA2-256=` 后面那串就是 `XRAY_ZIP_SHA256`

```bash
# 或者一条命令直接看
curl -sL https://github.com/XTLS/Xray-core/releases/download/v26.9.9/Xray-linux-64.zip.dgst \
  | grep '^SHA2-256='
```

3. 改完提交到 `main`，再打标签发版

> **注意**：Xray 官方把它的 Release 都标为 pre-release，这是它的惯例，
> 不影响我们——我们是按具体标签下载，不走 `releases/latest`。

### 升级前的建议

先在一台非关键 VPS 上验证新内核能正常使用（面板能看到流量、能正常连接），
确认没问题再全线更新。回滚只需把两个值改回旧版本，再发一个新版本号。

---

## 五、geo 数据：默认不动，需要时手动刷新

`bin/` 下 6 个 geo 数据文件（`geoip.dat`、`geosite.dat`，以及 IR、RU 两套）默认**沿用上一版**，
保证稳定、不随上游变动，也因此每次发布不需要重新下载 150MB 数据。

想刷新到上游最新版时，手动触发一次工作流并勾选 `refresh_geo`：

**Actions 页面 → `Build & Release` → 右侧 `Run workflow`**

| 字段 | 填什么 |
|---|---|
| `tag` | 一个新版本号，如 `v1.0.4` |
| `refresh_geo` | **勾上** |

刷新源和面板里「更新 Geo 文件」用的是同一批地址（Loyalsoldier、chocolate4u、runetfreedom）。

---

## 六、在网页上发版（不碰本地 git）

手边没电脑时也能发版：

**Actions 页面 → `Build & Release` → `Run workflow`**

| 字段 | 填什么 |
|---|---|
| `tag` | 想发布的版本号，如 `v1.0.4` |
| `refresh_geo` | 一般不勾 |

- 标签**已存在** → 直接用它发布
- 标签**不存在** → 自动在当前 `main` 最新提交上创建

---

## 七、安装新 VPS

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Arm0ne/Alien-Xui-Panel/main/install.sh)
```

脚本会自动：

1. 从 `releases/latest` 读取最新版本号
2. 按机器架构下载 `x-ui-linux-<arch>.tar.gz`（x86_64 对应 `amd64`）
3. **删除旧的 `/usr/local/x-ui/` 目录**，解压新包
4. 安装 systemd 服务、开放端口、设置时区，并打印面板信息

> **目前只提供 amd64（x86_64）资产。** 如果 VPS 是 ARM，安装会在下载环节 404。
> 需要 ARM 支持时，要额外准备一份 ARM 架构的 Xray 内核。

**支持的系统**（脚本内置校验）：Debian 11+、Ubuntu 20+、CentOS 8+、AlmaLinux 9+、
Rocky 9+、Fedora 36+、Arch、Manjaro、Armbian、Alpine、OpenSUSE Tumbleweed。

---

## 八、升级已有面板

```bash
x-ui            # 进管理菜单，选第 2 项「更新面板」
# 或直接
x-ui update
```

数据不会丢失 —— `/etc/x-ui/x-ui.db` 和证书都不在 `/usr/local/x-ui/` 里，不受影响。

只想临时替换单个二进制文件（比如只改了一行代码，不想走完整更新）：

```bash
systemctl stop x-ui
cp /usr/local/x-ui/x-ui /usr/local/x-ui/x-ui.bak
# 上传新的 x-ui 覆盖到 /usr/local/x-ui/x-ui
chmod +x /usr/local/x-ui/x-ui
systemctl start x-ui
```

> 忘了给执行权限是常见错误，症状是 `systemctl status` 显示 `203/EXEC`。

---

## 九、发布后如何验收

```bash
# 1. 确认 latest 指向新版本
curl -s https://api.github.com/repos/Arm0ne/Alien-Xui-Panel/releases/latest | grep tag_name

# 2. 确认下载链接可用（应返回 200）
curl -sIL -o /dev/null -w '%{http_code}\n' \
  https://github.com/Arm0ne/Alien-Xui-Panel/releases/download/v1.0.3/x-ui-linux-amd64.tar.gz

# 3. 下载并核对校验值（对照 Release 说明里的表格）
curl -sL -o /tmp/x.tgz \
  https://github.com/Arm0ne/Alien-Xui-Panel/releases/download/v1.0.3/x-ui-linux-amd64.tar.gz
sha256sum /tmp/x.tgz
tar -tvzf /tmp/x.tgz          # 权限应为 755/644，属主 0/0
```

面板启动后确认版本号显示正确：`x-ui -v` 应输出 `1.0.3`（不带 `v`）。

---

## 十、故障排查

| 症状 | 原因与处理 |
|---|---|
| 工作流第一步失败，提示标签格式 | 标签不是 `vX.Y.Z`。删掉重打：`git tag -d v1.0.3 && git push origin --delete v1.0.3` |
| `无法获取上一次发布包` | 仓库里没有任何历史 Release，geo 数据没有基线。先正常发布一个版本 |
| `Xray 内核校验和不符` | 上游重新上传了该版本文件，或版本号写错。到 XTLS 页面重新取 `.dgst` 里的 `SHA2-256` |
| `glibc 基线过新` | Zig 交叉编译没生效（可能 `CC` 被改动）。检查 workflow 编译步骤，不要改成直接 `go build` |
| `这是 CGO_ENABLED=0 的桩二进制` | `CGO_ENABLED` 被设成 0。sqlite 依赖 cgo，绝不能关 |
| 发布包缺少 `bin/xray-linux-amd64` | 内核下载或解压环节失败，往上翻日志 |
| 服务器装完后面板起不来，`203/EXEC` | 二进制没有执行权限，`chmod +x /usr/local/x-ui/x-ui` |
| 服务器上启动报 GLIBC 相关错误 | 编译时 glibc 基线过新，见上面那条 |
| 面板界面还是旧样子 | 浏览器缓存。`Ctrl+F5` 强刷；若仍未更新，确认版本号确实升了 |

---

## 十一、约定与「不要做的事」

### 不要做的事

- **不要勾 Pre-release，也不要点 Save draft** —— `releases/latest` 会跳过它们，一键脚本就拿不到新版本
- **不要删除所有历史 Release** —— 发布包里的 geo 数据是从上一版复用的，删光了就没有基线
- **不要改用 `ubuntu-latest` 自带 gcc 直接编译** —— 那会要求 glibc 2.39，Debian 11 / Ubuntu 20 装不上
- **不要关闭 CGO** —— sqlite 会退化成桩实现，面板一启动就崩
- **不要重发已存在的标签** —— `gh release create` 会失败报错，这是故意的保护
- **不要在标签推送之后才改 workflow** —— 标签触发使用的是**该标签提交上**的 workflow 文件。改完 workflow 必须先推到 main，再打新标签

### 工程决策备忘

**为什么用 Zig 而不是系统 gcc？**
产物需要 cgo（sqlite），而 CI 跑在 `ubuntu-latest`（glibc 2.39）。直接用系统 gcc 编译，
二进制会要求目标机 glibc ≥ 2.39，Debian 11（2.31）、Ubuntu 20.04（2.31）等老 VPS 全部装不上。
Zig 作为交叉编译器可把基线压到 **2.28**，覆盖脚本支持的全部系统。

**为什么内核要固定而不是跟随最新？**
新版本不一定更稳定。锁版本 + 官方 SHA256 校验 + 实跑 `xray -version`，
保证每次发布的内核都是字节级相同的那一个。

**为什么 geo 数据默认不刷新？**
同样是稳定性优先，且能省掉每次 150MB 的下载。需要更新时手动勾 `refresh_geo`。

**为什么标签即版本？**
避免「标签是 1.0.3、二进制自报 1.0.2」这种不一致。一个来源，不会忘。

---

## 附：相关文件与包结构

| 文件 | 作用 |
|---|---|
| `.github/workflows/release.yml` | 自动发布工作流（内核版本、Zig 版本都定义在其 `env:` 段） |
| `install.sh` | 新机器安装脚本，从 `releases/latest` 取包 |
| `x-ui.sh` | 服务器上的管理菜单（0–25 项） |
| `x-ui.service` | systemd 服务定义 |
| `config/version` | 版本号，由 CI 在编译前写入 |

**一键脚本取包的完整路径：**

```
https://github.com/Arm0ne/Alien-Xui-Panel/releases/download/<标签>/x-ui-linux-amd64.tar.gz
```

包内结构（缺任何一个都会出问题，因为安装脚本会先清空 `/usr/local/x-ui/`）：

```
x-ui/
├── x-ui                    ← 面板二进制 (755)
├── x-ui.sh                 ← 管理脚本   (755)
├── x-ui.service            ← systemd    (644)
└── bin/
    ├── xray-linux-amd64    ← Xray 内核  (755)
    ├── geoip.dat / geosite.dat
    ├── geoip_IR.dat / geosite_IR.dat
    ├── geoip_RU.dat / geosite_RU.dat
    └── LICENSE / README.md
```
