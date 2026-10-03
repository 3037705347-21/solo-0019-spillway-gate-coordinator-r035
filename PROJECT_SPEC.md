# 泄洪闸门操作协调服务

## 项目目标

该服务用于水工控制室协调泄洪闸门的命令申请、安全联锁复核和单次开度调整。系统在同一闸门上只允许一条已通过复核且尚未结束的执行命令，任何冲突都必须被显式拒绝，不能覆盖已经占用的闸门。

## 使用角色

- 运行协调员：登记闸门并提交开度调整命令。
- 安全复核员：提交联锁结论，决定命令是否通过。
- 闸门操作员：凭执行令牌启动命令并登记执行结果。

## 核心实体

- `gate`：闸门及其当前开度、运行状态、最大开度和活动命令。
- `gate_command`：一次开度调整申请，包含目标开度、申请原因、状态和执行令牌。
- `interlock_decision`：安全复核结论，状态为 clear 或 blocked。

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

闸门完成结构复审后，管理人员通过 `POST /gates/{id}/limit-revisions` 提交新的最大开度。请求必须携带结构复审依据编号、修订原因和修订人；只有空闲且无活动占用的闸门可以修订，且新限幅不得低于闸门当前物理开度。每次成功修订生成一条不可变的限幅记录，登记时的限幅恒为第 1 版。可通过 `GET /gates/{id}/limit-revisions` 读取完整修订历史。

### reconfirm-command-basis

限幅修订后，所有仍待复核的命令持有的是旧限幅依据。安全复核员提交 clear 结论前，命令必须由有权限的人员通过 `POST /commands/{id}/reconfirm` 重新确认到当前限幅版本；若冻结的目标开度已超出新限幅，重新确认被拒绝，命令只能改走 blocked 结论或被放弃。blocked 结论不占用闸门，在旧依据上仍然有效。

## 状态与规则

- `gate` 状态：idle、reserved、moving、faulted。
- `gate_command` 状态：pending、approved、rejected、executing、completed、aborted。
- 闸门限幅带版本号：登记为第 1 版，每次基于结构复审的修订单调递增；限幅历史只追加、不改写。
- 命令在创建时冻结限幅依据（版本号与当时最大开度）和起始开度。
- 命令读取结果通过 `basis_current` 说明其依据是否仍是当前限幅；终态命令永久保留自己的依据快照，不因后续修订而改变。
- 只有 pending 命令可以复核；clear 结论要求依据为当前版本，否则返回 `command_limit_basis_stale` 冲突且不改变任何状态；blocked 结论不受依据版本限制。
- 只有 pending 命令可以重新确认依据；重新确认按当前限幅重新校验目标开度与起始开度。
- 只有 approved 命令可以执行，且执行令牌必须匹配。
- 限幅修订与 clear 复核在同一把仓库互斥锁内串行：占用中的闸门（reserved、moving、faulted）拒绝修订，修订先成则旧依据 clear 被拒，二者只有一个明确结果。
- clear 复核与闸门占用必须同时成功；任一步失败时两者都不写入。
- blocked 复核使命令 rejected，不会占用闸门。
- 模拟故障会令命令 aborted、闸门 faulted，并保留最终开度。
- 已完成或已中止的命令不能再次执行，其历史事实不能被限幅修订改写。

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
- `POST /commands/{id}/reconfirm`
- `POST /commands/{id}/interlock`
- `POST /commands/{id}/execute`

错误响应统一为：

```json
{"code":"stable_error_code","message":"可读说明"}
```

## 验证计划

本初始化基线刻意不生成单元测试、夹具或浏览器测试文件，也不声明 `test_command`。后续任务阶段负责补充测试，并在该阶段建立红绿验证。当前每条工作流通过 `cmd/workflowcheck` 启动真实 HTTP 服务并调用公开接口，校验成功状态、持久化结果和关键失败分支。
