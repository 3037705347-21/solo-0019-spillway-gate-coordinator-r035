# 泄洪闸门操作协调服务

这是一个无外部依赖的 Go 后端，用本地内存仓库协调闸门登记、开度调整命令、安全联锁复核和执行结果。服务强调单闸门活动命令互斥、原子状态转换、执行令牌生命周期和稳定错误传播。

## 运行

```sh
go build ./...
go run ./cmd/server --addr 127.0.0.1:18119
```

健康入口为 `GET http://127.0.0.1:18119/health`。

## API

```text
POST /gates
GET  /gates/{id}
POST /gates/{id}/limit-revisions
GET  /gates/{id}/limit-revisions
POST /commands
GET  /commands/{id}
POST /commands/{id}/interlock
POST /commands/{id}/execute
```

登记闸门示例：

```json
{
  "id": "gate-east",
  "name": "东侧泄洪闸",
  "max_aperture": 4.5,
  "initial_aperture": 0.5
}
```

提交命令示例：

```json
{
  "gate_id": "gate-east",
  "target_aperture": 2.25,
  "reason": "上游水位调整",
  "requested_by": "dispatcher-lin"
}
```

联锁复核示例：

```json
{
  "verdict": "clear",
  "verifier": "safety-wu",
  "note": "上下游条件满足",
  "limit_revision": 2
}
```

结构复审后修订最大开度示例：

```json
{
  "new_max_aperture": 5.5,
  "basis": "structural-review-2026-09",
  "reason": "结构复审确认允许更高开度",
  "requested_by": "manager-qin"
}
```

开度限制规则：

- 闸门登记时限制修订号为 1，每次成功修订加一，修订记录只追加，可通过 `GET /gates/{id}/limit-revisions` 读取。
- 只有 idle 且没有活动命令的闸门可以修订；新限值不得低于闸门当前开度。
- 命令创建时冻结当时的限值和修订号。限值变更后，待复核命令的 clear 必须显式确认新修订号；未携带 `limit_revision` 时按创建时的旧依据处理，会返回 `stale_limit_revision` 冲突。目标超出新限值的命令无法 clear，只能 blocked。
- 命令读取响应同时给出冻结的 `limit_max_aperture`、当前 `current_max_aperture` 和 `limit_current` 标记；completed/aborted/rejected 的事实不会被修订改写。

执行示例：

```json
{
  "operator": "operator-chen",
  "token": "复核响应中的 execution_token",
  "simulate_fault": false
}
```

## 目录

- `cmd/server`：服务进程。
- `cmd/workflowcheck`：四条工作流的真实 HTTP 检查入口。
- `internal/bootstrap`：依赖装配。
- `internal/contracts`：公开数据契约。
- `internal/coordination`：业务工作流。
- `internal/domain`：实体、不变量和状态机。
- `internal/httpapi`：路由、解码和错误映射。
- `internal/storage`：互斥保护的内存仓库。

## 工作流检查

```sh
go run ./cmd/workflowcheck --workflow register-gate
go run ./cmd/workflowcheck --workflow request-gate-command
go run ./cmd/workflowcheck --workflow review-safety-interlock
go run ./cmd/workflowcheck --workflow execute-gate-command
go run ./cmd/workflowcheck --workflow revise-gate-limit
```

这些检查会创建临时 HTTP 服务并调用真实路由。它们不调用外部网络，也不依赖数据库。开度限制修订检查包含多轮修订与复核并发，验证同一临界区内只有一个赢家。

## 测试状态

测试有意延后。当前交付仅包含初始化基线和生产级 smoke 检查；后续任务阶段会添加自动化测试与红绿验证，不应把当前没有测试文件视为项目未完成。
