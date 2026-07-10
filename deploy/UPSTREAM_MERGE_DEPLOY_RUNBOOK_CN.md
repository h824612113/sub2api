# 上游合并与生产部署运行手册

本文档用于将 `upstream/main` 合并到当前商业版本，并在不改动 PostgreSQL、Redis 数据容器的前提下部署应用。目标是让下一次合并可以沿用固定检查项，不再重复确认分支、测试和部署命令。

## 当前生产约定

- 上游仓库：`upstream`（`Wei-Shaw/sub2api`）
- 商业仓库：`origin`（`h824612113/sub2api`）
- 生产 Compose：`deploy/docker-compose.yml`
- 生产应用镜像：`sub2api-commercial:codex-commercial-relay-mvp`
- 应用容器：`sub2api`
- 数据容器：`sub2api-postgres`、`sub2api-redis`
- 生产健康地址：`http://127.0.0.1:8080/health`、`https://ai.cyberhz.com/health`
- 备份目录：`/root/sub2api-backups`

不要以本地 `main` 作为商业版本基线。它可能长期落后于 `origin/main`，也不包含当前商业改动。每次都应从当前实际生产分支创建新的合并分支。

## 快速流程

1. 记录当前分支、提交、镜像和容器状态。
2. 拉取 `upstream/main`，从当前生产分支创建日期命名的合并分支。
3. 合并上游，解决冲突后检查商业逻辑不变量。
4. 并行执行前端类型检查、后端编译和专项测试。
5. 提交合并结果，但先不要部署。
6. 获得部署确认后先备份，再构建镜像。
7. 只重建 `sub2api`，不要重建 PostgreSQL 或 Redis。
8. 检查健康、日志、公网页面和部署前后数据计数。

## 1. 合并前记录

```bash
git status --short --branch
git branch --show-current
git rev-parse HEAD
git remote -v
docker inspect --format='{{.Image}}' sub2api
docker ps --filter name=sub2api --format='{{.Names}}\t{{.Status}}'
```

工作区可能存在运营截图等未跟踪文件。不要使用 `git clean`，也不要删除或覆盖与本次合并无关的文件。

## 2. 拉取并合并上游

```bash
git fetch --prune upstream main
git rev-list --left-right --count HEAD...upstream/main
git log --oneline HEAD..upstream/main

git switch -c merge/upstream-main-$(date -u +%Y%m%d)
git merge --no-ff upstream/main
```

合并前如需只读预判冲突，可运行：

```bash
git merge-tree "$(git merge-base HEAD upstream/main)" HEAD upstream/main
```

冲突解决完成后：

```bash
git diff --check
git status --short
git log --oneline --decorate --max-count=12
```

## 3. 商业逻辑不变量

每次上游合并后都必须确认以下逻辑仍然存在。详细背景见 `docs/SUBSCRIPTION_BUNDLE_GROUPS_CHANGE_RECORD.md`。

- 同一套餐的主分组和候选分组共享周/月额度池。
- 用户订阅页按共享池聚合展示，不能重复累计同池用量。
- 购买和续费会发放或延长 bundle 内全部订阅分组。
- 正数订阅兑换码会发放整个 bundle。
- 负数兑换码会扣减整个 bundle，并同步清理订阅 L1 缓存。
- 退款最终扣减会处理整个 bundle；退款失败回滚也必须恢复整个 bundle。
- 购买、续费、充值跳转仍使用站内预期入口，不能被上游路由覆盖。
- 用户侧不能显示 `quota_pool_*`、`subscription_bundle_groups` 等内部配置行。

重点复查文件：

```text
backend/internal/service/group.go
backend/internal/service/subscription_quota_pool.go
backend/internal/service/subscription_service.go
backend/internal/service/billing_cache_service.go
backend/internal/service/payment_fulfillment.go
backend/internal/service/payment_refund.go
backend/internal/service/redeem_service.go
frontend/src/views/user/PaymentView.vue
frontend/src/components/payment/SubscriptionPlanCard.vue
```

## 4. 快速验证门槛

前端：

```bash
cd frontend
pnpm typecheck
cd ..
```

后端编译和上游兼容层：

```bash
cd backend
go test ./internal/pkg/apicompat -count=1
go test ./internal/handler -run '^TestOpsCaptureWriter' -count=1
go test ./internal/service -run '^$' -count=1
```

订阅共享额度、购买发放、兑换缓存和退款专项：

```bash
go test ./internal/service -run 'Test(GroupQuotaPoolKey|AggregatePooledSubscriptionQuota|SubscriptionServiceList_NormalizesPooledDisplayPerUser|SubscriptionServiceValidateAndCheckLimits|BillingCacheServiceCheckSubscriptionEligibility|ExtendSubscriptionBundle|ExecuteSubscriptionFulfillment|RedeemService_InvalidateRedeemCaches_SubscriptionBundleL1AfterCommit|ApplyRefundFinalDeductionDeductsAllBundleSubscriptions)' -count=1
cd ..
```

涉及共享服务、数据库迁移或大范围上游修改时，再执行完整测试：

```bash
cd backend
go test ./...
```

只有测试和差异检查完成后才提交合并结果。测试通过不等于已授权部署。

## 5. 部署前备份

```bash
SUB2API_BACKUP_DIR=/root/sub2api-backups \
SUB2API_BACKUP_RETENTION_DAYS=14 \
./deploy/backup-sub2api.sh
```

脚本会生成数据库 dump、全局角色、部署配置、容器信息、表计数和校验清单。找到最新目录并校验：

```bash
LATEST_BACKUP="$(ls -1dt /root/sub2api-backups/20* | head -n 1)"
cd "${LATEST_BACKUP}"
sha256sum -c SHA256SUMS
cat table-counts.txt
cd /root/sub2api
```

备份必须包含 `sub2api.dump`，且所有哈希均为 `OK`。保留 `table-counts.txt`，部署后用相同查询对比用户、账户、API Key、订阅等数量。

## 6. 构建与切换

先保存旧镜像 ID，方便回滚：

```bash
OLD_IMAGE="$(docker inspect --format='{{.Image}}' sub2api)"
docker tag "${OLD_IMAGE}" "sub2api-commercial:rollback-$(date -u +%Y%m%dT%H%M%SZ)"
```

服务器安装了可用的 `buildx` 时使用 BuildKit：

```bash
DOCKER_BUILDKIT=1 docker build \
  -t sub2api-commercial:codex-commercial-relay-mvp \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -f Dockerfile .
```

如果提示 `buildx component is missing or broken`，使用传统构建器：

```bash
DOCKER_BUILDKIT=0 docker build \
  -t sub2api-commercial:codex-commercial-relay-mvp \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -f Dockerfile .
```

确认新镜像 ID 与旧镜像不同，再只重建应用：

```bash
docker inspect --format='{{.Id}}' sub2api-commercial:codex-commercial-relay-mvp
docker compose -f deploy/docker-compose.yml up -d --no-deps --force-recreate sub2api
```

当前是单应用实例，强制重建时网页可能短暂不可用数秒。要做到无感切换，需要另行实施双实例蓝绿部署和反向代理健康切流。

## 7. 部署后验证

```bash
docker inspect --format='{{.Image}}\t{{.State.Status}}\t{{.State.Health.Status}}\t{{.RestartCount}}' sub2api
docker ps --filter name=sub2api --format='{{.Names}}\t{{.Status}}'
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://ai.cyberhz.com/health
curl -fsS -o /dev/null -w '%{http_code}\n' https://ai.cyberhz.com/dashboard
```

日志检查：

```bash
docker logs --since 15m sub2api 2>&1 | rg -i 'panic|fatal|migration|failed to initialize|checksum mismatch|database.*(error|failed)|redis.*(error|failed)|payment.*(error|failed)|subscription.*(error|failed)|status_code.: (502|503)'
```

判断规则：

- `panic`、启动失败、迁移失败、数据库或 Redis 连接失败：阻断部署，立即评估回滚。
- `/api/v1/...` 用户、订阅、支付接口 5xx：阻断部署并定位。
- `/v1/responses`、`/v1/embeddings`、`/v1/models` 的 502/503：继续按账号、模型和上游错误定位，不能直接认定为网页或数据故障。
- `EMAIL_NOT_CONFIGURED`：当前表示邮件服务未配置，会影响订阅到期提醒，但不影响额度和支付主流程。

部署后再次读取核心表计数。用户、账户、API Key、订阅数量不能无故减少，`usage_logs` 正常情况下会继续增长。

## 8. 回滚

找到部署前保存的 rollback 标签：

```bash
docker images --format='{{.Repository}}:{{.Tag}}\t{{.ID}}' | rg 'sub2api-commercial:rollback-'
```

将选定旧镜像重新标记为生产标签，然后只重建应用：

```bash
ROLLBACK_IMAGE_ID="填写上一步选择的镜像 ID"
docker tag "${ROLLBACK_IMAGE_ID}" sub2api-commercial:codex-commercial-relay-mvp
docker compose -f deploy/docker-compose.yml up -d --no-deps --force-recreate sub2api
```

如果新版本已经执行了不兼容的数据迁移，不能只回滚镜像；需要先评估补偿 SQL 或从部署前备份恢复。

## 9. 每次合并记录模板

```text
日期：
生产基线提交：
upstream/main 提交：
合并提交：
保留的商业提交/逻辑：
冲突文件：
前端检查：
后端检查：
订阅/支付专项检查：
备份目录：
旧镜像 ID：
新镜像 ID：
健康检查：
数据计数对比：
已知非阻断告警：
回滚标签：
```
