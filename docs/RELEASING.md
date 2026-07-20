# base-code 发版与安装器分发指南

> 面向作者本人的运维手册：讲清楚「一次发版背后发生了什么」，以及「以后怎么发新版、brew/scoop 怎么自动更新」。

---

## 一、一张图看懂全流程

```
你在本地做的事                GitHub 自动做的事（CI）                 用户装到的东西
────────────                ──────────────────────               ──────────────
                             ┌──────────────────────────┐
git tag v0.2.0    ──push──▶  │  主仓库 base-code 的 CI    │
git push --tags              │  (.github/workflows/       │
                             │   release.yml) 启动        │
                             │        │                   │
                             │        ▼                   │
                             │  goreleaser 读             │
                             │  .goreleaser.yaml          │
                             │        │                   │
                             │   编 6 平台二进制 + 打包    │
                             │        │                   │
                             │   ┌────┼─────────┐         │
                             │   ▼    ▼         ▼         │
                             │ Release  cask   manifest   │
                             └───┼──────┼────────┼────────┘
                                 │      │        │
                                 ▼      ▼        ▼
                          base-code   homebrew  scoop
                          /releases   -tap      -bucket
                                 │      │        │
                                 ▼      ▼        ▼
                            手动下载  brew装   scoop装
```

**核心心法**：配好一次之后，**你以后发版只需要两条命令**（打 tag + push），剩下全自动。下面每一块解释「为什么需要它」。

---

## 二、三个仓库 + 一个 PAT，各是干嘛的

| 名字 | 类型 | 作用 | 谁写它 |
|---|---|---|---|
| **`dhslegen/base-code`** | 主仓库 | 放源码、`.goreleaser.yaml`、CI 配置 | 你 |
| **`dhslegen/homebrew-tap`** | 分发仓库 | 放 Homebrew 的安装清单 `Casks/base-code.rb` | CI 自动写 |
| **`dhslegen/scoop-bucket`** | 分发仓库 | 放 Scoop 的安装清单 `bucket/base-code.json` | CI 自动写 |
| **`TAP_GITHUB_TOKEN`** | 密钥(secret) | 让主仓库的 CI 有权限往上面两个分发仓库写文件 | 你建一次 |

**为什么要分三个仓库？**
- 二进制文件（几 MB）放在主仓库的 **Release** 里。
- 但 `brew` / `scoop` 不直接下载二进制——它们先读一个**「安装清单」**（记录「哪个版本、去哪下、文件校验码是多少」），再照着清单去 Release 下载。
- Homebrew 规定：清单必须放在一个**独立的、名字以 `homebrew-` 开头的仓库**里（这就是 `homebrew-tap`）。Scoop 同理需要一个 bucket 仓库。
- 所以是「主仓库出二进制，两个分发仓库出清单」。

---

## 三、PAT 是什么、为什么非它不可

**PAT = Personal Access Token（个人访问令牌）**，相当于一把「代表你身份的钥匙」。

**为什么普通的 CI 令牌不够用？**
- GitHub 给每次 CI 自动发一把钥匙叫 `GITHUB_TOKEN`，但它**只能开当前仓库的锁**（安全设计，防止一个仓库的 CI 乱改别的仓库）。
- 而我们的 CI 跑在 `base-code` 里，却要往 `homebrew-tap` 和 `scoop-bucket` **写文件**——跨仓库了，`GITHUB_TOKEN` 开不了那两把锁。
- 所以需要你手动建一把**能开那两个分发仓库的钥匙**（PAT），交给 CI 用。这把钥匙存在主仓库的 secret 里，名字叫 `TAP_GITHUB_TOKEN`。

**这把钥匙的权限被刻意限制到最小**：
- 只能碰 `homebrew-tap` + `scoop-bucket` 两个仓库（不能碰主仓库、不能碰你别的仓库）。
- 只有 **Contents: Read and write**（只能读写文件，不能删仓库、不能改设置）。
- 万一泄露，损失被圈死在这两个「反正是公开的」分发仓库里。

**⚠️ PAT 会过期**（当初设了 365 天，约 **2027-07-15** 到期）。到期后 CI 发版会报 `401`。到时按下面「PAT 更换」一节重建即可。

---

## 四、【最常用】以后怎么发一个新版本

假设你改了代码、想发布 `v0.2.0`：

```bash
cd /Users/zhaowenhao/Developer/Work/BackEnd/General-Component-backend/base-code-go

# 1. 提交代码改动（正常开发）
git add -A
git commit -m "feat: 某某新功能"
git push origin main

# 2. 打一个版本 tag（注意 v 开头，必须是 vX.Y.Z 格式）
git tag -a v0.2.0 -m "v0.2.0 — 某某新功能"

# 3. 把 tag 推上去 —— 这一步会触发 CI 自动发版
git push origin v0.2.0
```

**推完 tag 就没你的事了。** GitHub 会自动：编 6 平台二进制 → 建 Release → 更新 homebrew cask → 更新 scoop manifest。

### 想盯着 CI 跑完 + 验证（可选）

```bash
# 看 CI 状态
gh run list --workflow=release.yml --limit 1

# 盯它跑完（约 2 分钟）
gh run watch $(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId') --exit-status

# 验证清单已更新到新版本
gh api repos/dhslegen/homebrew-tap/contents/Casks/base-code.rb --jq '.content' | base64 -d | grep version
gh api repos/dhslegen/scoop-bucket/contents/bucket/base-code.json --jq '.content' | base64 -d | grep version
```

### ⚠️ 发版红线（否则 CI 会失败或不触发）
- tag **必须 `v` 开头**（`v0.2.0` ✅；`0.2.0` ❌ 不触发；`tutorial-v1` ❌ 不匹配 `v*`）。
- tag 必须是合法 semver（`vX.Y.Z`），否则 goreleaser 会中止。
- 发 tag 前先 `git push origin main`，确保 tag 指向的提交在远端能被 CI 拉到。

---

## 五、版本号怎么选（语义化版本 semver）

格式 `v主版本.次版本.修订号`，当前最新是 **v0.4.0**。下一个怎么选：

| 你做了什么 | 版本号怎么加 | 例子 |
|---|---|---|
| 修 bug、小改动，不影响用法 | 修订号 +1 | v0.4.0 → **v0.4.1** |
| 加了新功能，老用法照常能用 | 次版本 +1，修订号归 0 | v0.4.0 → **v0.5.0** |
| 破坏性改动（老用法不兼容）/ 首个正式稳定版 | 主版本 +1 | v0.4.0 → **v1.0.0** |

> `0.x.y` 阶段表示「还在早期、接口可能变」；发 `v1.0.0` 相当于宣布「稳定了，可放心依赖」。

---

## 六、用户怎么装、怎么更新（尤其 brew 更新原理）

### 安装

```bash
# macOS / Linux
brew install dhslegen/tap/base-code

# Windows
scoop bucket add dhslegen https://github.com/dhslegen/scoop-bucket
scoop install base-code
```

### brew 如何拿到你发的新版本（更新原理）

用户装完后，你发了 `v0.2.0`，用户这样升级：

```bash
brew update                 # ① 刷新所有 tap 的最新清单（含你的 homebrew-tap）
brew upgrade base-code      # ② 对比已装版本 vs 清单里的新版本，有新就升
```

**原理链**（为什么用户 `brew upgrade` 就能拿到）：
```
你 push tag v0.2.0
   └─▶ CI 把 homebrew-tap/Casks/base-code.rb 里的 version 改成 "0.2.0"
          └─▶ 用户 brew update：git 拉取 homebrew-tap 最新提交，看到 version 变了
                 └─▶ 用户 brew upgrade base-code：发现本地是 0.1.3、清单是 0.2.0 → 下载升级
```

- 你**不需要通知用户**，也不需要手动改任何清单——`brew update` 本质是 `git pull` 你的 tap 仓库。
- 用户想一次升所有软件：`brew upgrade`（不带名字）。

### scoop 更新（对称）
```powershell
scoop update              # 刷新所有 bucket 清单
scoop update base-code    # 升级这个包
```

---

## 七、出问题怎么查（FAQ）

| 症状 | 原因 | 解决 |
|---|---|---|
| CI 里 cask/manifest 步骤报 **401/403** | PAT 过期或权限不足 | 见下「PAT 更换」；确认 PAT 选了两个分发仓库 + Contents:RW |
| push tag 后 **CI 完全没触发** | tag 不是 `v` 开头 | 删掉重打：`git tag -d 名字 && git push origin :名字`，再用 `vX.Y.Z` |
| 用户 `brew install` 报 **找不到** | tap 没加或 formula 名错 | 确认 `brew tap dhslegen/tap` 成功，或用完整三段 `dhslegen/tap/base-code` |
| 用户 `command not found: base-code`（走 go install 的） | `~/go/bin` 不在 PATH | 改用 brew/scoop，或把 `~/go/bin` 加进 PATH |
| goreleaser 配置报 **deprecated** | goreleaser 升级废弃了旧字段 | 本地 `goreleaser check` 看提示，查 goreleaser.com/deprecations 迁移 |

### PAT 更换（到期或泄露时）
1. 去 https://github.com/settings/personal-access-tokens 删掉旧 token。
2. 按当初的设置重建：Resource owner `dhslegen`，只选 `homebrew-tap` + `scoop-bucket`，权限 Contents: Read and write，有效期 365 天。
3. 复制新 token，更新主仓库 secret：
   ```bash
   gh secret set TAP_GITHUB_TOKEN --repo dhslegen/base-code --body "<新PAT>"
   ```
4. 下次发版即生效。

---

## 八、速查表（发一个新版本，就记这个）

```bash
git add -A && git commit -m "feat: xxx" && git push origin main
git tag -a v0.2.0 -m "v0.2.0 — xxx"
git push origin v0.2.0
# 完事。CI 自动建 Release + 更新 brew cask + 更新 scoop manifest。
# 用户端：brew update && brew upgrade base-code
```

---

## 附：本次搭建时踩过的两个坑（备忘）
- goreleaser **v2.16 移除了 `brews`**，改用 `homebrew_casks`（cask 落 `Casks/`）；未签名二进制需 `xattr -dr com.apple.quarantine` 的 post-install hook 免被 macOS Gatekeeper 拦。
- goreleaser 的 **`scoops` 默认推仓库根目录**，新版 scoop 只认 `bucket/` 子目录，需显式配 `directory: bucket`。
