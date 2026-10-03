# 泄洪闸门操作协调服务

## 项目目标

该服务用于水工控制室协调泄洪闸门的命令申请、安全联锁复核和单次开度调整。系统在同一闸门上只允许一条已通过复核且尚未结束的执行命令，任何冲突都必须被显式拒绝，不能覆盖已经占用的闸门。

## 使用角色

- 运行协调员：登记闸门并提交开度调整命令。
- 安全复核员：提交联锁结论，决定命令是否通过。
- 闸门操作员：凭执行令牌启动命令并登记执行结果。
- 管理人员：凭结构复审依据修订闸门最大开度。

## 核心实体

- `gate`：闸门及其当前开度、运行状态、最大开度、开度限制修订号和活动命令。
- `gate_command`：一次开度调整申请，包含目标开度、申请原因、状态、执行令牌以及创建时冻结的开度限制。
- `interlock_decision`：安全复核结论，状态为 clear 或 blocked，并记录所确认的限制修订号。
- `limit_revision`：一条只追加的最大开度修订记录，登记基线序号为 1，后续修订单调递增。

## 业务流程

### register-gate

运行协调员通过 `POST /gates` 登记闸门。服务校验标识、名称、最大开度和初始开度，写入空闲状态；重复标识返回稳定冲突错误，不覆盖原记录。

### request-gate-command

运行协调员通过 `POST /commands` 提交开度调整命令。服务读取闸门当前开度和限制，校验目标开度与申请信息，创建待复核命令并冻结起始开度。

### review-safety-interlock

安全复核员通过 `POST /commands/{id}/interlock` 提交复核结论。clear 结论会原子占用闸门并签发一次执行令牌；blocked 结论会把命令置为 rejected。其他命令先占用同一闸门时，当前 clear 结论返回冲突且不改变闸门。

### execute-gate-command

闸门操作员通过 `POST /commands/{id}/execute` 提交执行令牌。服务校验命令、令牌、闸门占用关系和目标开度，在临界区内完成 approved、executing、completed 或 aborted 的转换，并同步更新闸门状态。令牌在命令结束后失效，不能复用。

### revise-gate-limit

管理人员在结构复审完成后通过 `POST /gates/{id}/limit-revisions` 修订最大开度，必须提供复审依据、修订原因和修订人。只有 idle 且没有活动命令的闸门可以修订；新限值必须大于零且不低于闸门当前开度。每次修订在同一临界区内推进闸门的开度限制修订号，并追加一条只增不改的审计记录；登记时自动生成序号为 1 的基线记录，可通过 `GET /gates/{id}/limit-revisions` 读取完整历史。

修订对各阶段命令的解释：

- 新命令：创建时冻结当时的最大开度与限制修订号，目标超出当前限值直接拒绝。
- 待复核命令：复核请求必须确认当前限制修订号；未携带时视为坚持创建时的旧依据。限值一旦变化，旧依据的 clear 返回冲突，必须按新修订号重新确认；目标超出新限值的命令即使重新确认也不能 clear，只能 blocked 或另建新命令。
- 活动占用：已通过复核的命令（approved/executing）占用闸门，期间禁止修订限值，因而其执行依据不会被改写；执行仍按复核时确认的限值校验最终开度。
- 历史读取：已完成、已中止和已拒绝的事实不可改写，命令视图同时保留创建时冻结的限值、修订号和闸门当前限值与 `limit_current` 标记，历史可据此正确解释。

## 状态与规则

- `gate` 状态：idle、reserved、moving、faulted。
- `gate_command` 状态：pending、approved、rejected、executing、completed、aborted。
- 闸门带单调递增的开度限制修订号，登记为 1，每次成功修订加一；限制修订记录只追加、不可改写。
- 命令在创建时冻结起始开度、当时的最大开度和限制修订号；复核结论记录所确认的修订号。
- 只有 pending 命令可以复核，且 clear 必须确认闸门当前限制修订号，目标开度不得超过当前限值。
- 只有 approved 命令可以执行，且执行令牌必须匹配。
- clear 复核与闸门占用必须同时成功；任一步失败时两者都不写入。
- blocked 复核使命令 rejected，不会占用闸门。
- 模拟故障会令命令 aborted、闸门 faulted，并保留最终开度。
- 已完成或已中止的命令不能再次执行。
- 只有 idle 且无活动命令的闸门可以修订开度限制；修订与复核针对同一闸门并发时，在同一互斥临界区内决出唯一结果。

## 模块边界

- `cmd/server`：进程入口、监听地址和退出控制。
- `internal/httpapi`：HTTP 路由、JSON 解码、状态码映射。
- `internal/coordination`：工作流编排、命令标识和执行令牌签发。
- `internal/domain`：实体、不变量和状态转换规则。
- `internal/storage`：带互斥保护的内存仓库和原子写入。
- `internal/contracts`：API 请求、响应和稳定错误结构。
- `internal/bootstrap`：运行时依赖装配。

## 接口

- `GET /health`
- `POST /gates`
- `GET /gates/{id}`
- `POST /gates/{id}/limit-revisions`
- `GET /gates/{id}/limit-revisions`
- `POST /commands`
- `GET /commands/{id}`
- `POST /commands/{id}/interlock`
- `POST /commands/{id}/execute`

错误响应统一为：

```json
{"code":"stable_error_code","message":"可读说明"}
```

## 验证计划

本初始化基线刻意不生成单元测试、夹具或浏览器测试文件，也不声明 `test_command`。后续任务阶段负责补充测试，并在该阶段建立红绿验证。当前每条工作流通过 `cmd/workflowcheck` 启动真实 HTTP 服务并调用公开接口，校验成功状态、持久化结果和关键失败分支；开度限制修订工作流额外用多轮并发请求验证修订与复核之间必有唯一赢家。
