# 命令执行与人工审核设计

## 目标

在现有 daemon 骨架上继续扩展，让它可以执行一个已配置的命令、持久化执行状态，并在需要人工判断时停止自动推进。

这一阶段还不是任务队列系统，而是一个带显式人工控制的“单工作项 runner”。

## 范围

本设计新增：

- 用真实命令执行替代当前仅写日志的 ticker 行为
- 一个持久化的“当前工作项”
- 明确的 runner 状态机
- 人工审核状态
- `approve`、`reject`、`resume` 控制命令
- 可供人工查看的执行结果持久化

本设计不新增：

- SQLite 或多任务队列
- 自动规划下一条命令
- 多任务并发执行
- 远程 API
- 第一版中的 shell 管道、重定向和任意 shell 解析

## 为什么这样设计

当前项目已经有一个可运行的 daemon 骨架：

- 前台/后台生命周期
- health endpoint
- PID 和地址文件
- status 和 stop 命令
- 周期性 ticker 循环

下一步最有学习价值的功能，不是“每个 tick 永远重复跑同一个命令”。那样会直接落入你前面提到的问题：前一次结果已经不对，后一次还继续跑，问题只会越滚越大。

更合理的下一步应该是一个最小任务 runner：

- 一次只处理一个工作项
- 只有一个明确状态
- 一次只发生一个状态迁移
- 在关键决策点允许人工介入

## 推荐模型

采用**单工作项状态机**。

daemon 始终只持有一个当前工作项。每次 tick 只在当前状态允许推进时才继续。

这是最小但足够的模型，能支持：

- 成功时自动执行
- 失败时停止
- 需要人工检查时停止
- 只有显式人工操作后才恢复推进

## 工作项模型

工作项是一个持久化文档，包含：

- 命令可执行文件
- 命令参数
- 当前状态
- 上次开始时间
- 上次结束时间
- 上次退出码
- 上次 stdout
- 上次 stderr
- 上次错误摘要
- 上次人工审核动作

它可以放在一个本地状态文件里，例如：

- `~/.mydaemon/task-state.json`

这个文件和下面这些运行时文件分开：

- `~/.mydaemon/mydaemon.pid`
- `~/.mydaemon/mydaemon.log`
- `~/.mydaemon/mydaemon.addr`

## 状态机

工作项使用以下状态：

- `idle`
- `running`
- `needs_review`
- `blocked`
- `completed`

### 状态含义

- `idle`
  - 下一个 tick 可以执行
- `running`
  - daemon 当前正在执行命令
- `needs_review`
  - 命令已经执行完，但在继续前必须人工判断
- `blocked`
  - 执行失败，或被人工明确拒绝
- `completed`
  - 当前工作项已经成功完成

### Tick 行为

每个 ticker 周期按下面规则推进：

- 如果状态是 `idle`，启动一次执行
- 如果状态是 `running`，什么都不做
- 如果状态是 `needs_review`，什么都不做
- 如果状态是 `blocked`，什么都不做
- 如果状态是 `completed`，什么都不做

这样可以防止“无脑重复执行”。

## 命令配置方式

第一版推荐使用：

- 可执行文件路径或命令名
- 参数数组

这样可以避免一上来就引入 shell quoting、管道、重定向和 shell 注入复杂度。

例子：

```json
{
  "command": "date",
  "args": ["+%F %T"]
}
```

命令来源可以是单独的本地配置文件，也可以直接写在 `task-state.json` 里。关键点是执行必须走 `exec.CommandContext(command, args...)`，而不是 shell 解释。

## 结果约定

执行命令通过退出码表达结果类型：

- `0` = 成功
- `10` = 需要人工审核
- 其他任意非 `0` = 失败

daemon 需要记录：

- 退出码
- stdout
- stderr
- 结束时间

然后按下面规则迁移状态：

- `0` -> `completed`
- `10` -> `needs_review`
- 其他非 `0` -> `blocked`

这样在第一版就能有明确约定，而不需要更复杂的协议。

## 人工控制命令

CLI 增加 3 个控制命令：

- `approve`
- `reject`
- `resume`

### 语义

- `approve`
  - 只允许从 `needs_review` 使用
  - 记录最近一次审核结果为 approved
  - 状态迁移到 `completed`
- `reject`
  - 只允许从 `needs_review` 使用
  - 记录最近一次审核结果为 rejected
  - 状态迁移到 `blocked`
- `resume`
  - 只允许从 `blocked` 使用
  - 清除阻塞状态
  - 状态迁移回 `idle`

第一版不要让 daemon 猜“人是不是已经看过了”，必须通过显式命令表达。

## 执行规则

daemon 对当前工作项一次只执行一个命令。

执行流程：

1. 读取任务状态
2. 如果状态不是 `idle`，直接返回
3. 把状态写成 `running`
4. 用 context 执行命令
5. 捕获 stdout、stderr、退出码和时间戳
6. 持久化结果
7. 把状态迁移到 `completed`、`needs_review` 或 `blocked`

如果 daemon 在执行过程中崩溃，下一次启动时对 `running` 状态要保守处理。这个阶段最简单的规则是：

- 启动时如果发现持久化状态还是 `running`，直接改写成 `blocked`

这和 Multica 的大原则一致：不确定的中途执行不能被当成成功继续推进。

## 日志

daemon 仍然要把运行日志写到 `~/.mydaemon/mydaemon.log`，至少包括：

- 命令开始
- 命令结束
- 退出码
- 状态迁移

结构化真相来源是状态文件，日志只是运行历史。

## 错误处理

失败类型主要有：

- 命令无法启动
- 命令执行失败
- 命令主动要求人工审核
- 状态文件读写失败

规则：

- 状态持久化失败对本次 tick 是 fatal，并且要记录明显日志
- 命令启动失败 -> `blocked`
- 命令非零退出 -> `blocked`
- 审核退出码 -> `needs_review`

## 测试策略

第一轮实现至少覆盖这些测试：

1. `idle` 工作项在退出码为 `0` 时执行并进入 `completed`
2. 退出码 `10` 时进入 `needs_review`
3. 其他非零退出码时进入 `blocked`
4. `approve` 让 `needs_review` -> `completed`
5. `reject` 让 `needs_review` -> `blocked`
6. `resume` 让 `blocked` -> `idle`
7. `needs_review` 状态下 ticker 不再执行
8. `blocked` 状态下 ticker 不再执行
9. 启动时把持久化的 `running` 改写为 `blocked`

测试里应使用 fake executor 或可注入执行层，这样状态机行为可以被确定性验证。

## 文件结构变化

推荐新增或修改这些最小文件：

- `internal/task/runner.go`
  - 从“只写日志的 tick 行为”升级为命令执行编排
- `internal/task/state.go`
  - 定义工作项状态结构和 load/save 帮助函数
- `internal/task/executor.go`
  - 定义一个小的命令执行抽象
- `cmd/mydaemon/main.go`
  - 增加 `approve`、`reject`、`resume`
- `internal/app/app.go`
  - 启动时恢复并处理残留的 `running` 状态

文件拆分在规划阶段可以微调，但边界建议保持：

- 生命周期逻辑放在 `internal/app`
- 工作项状态和迁移放在 `internal/task`
- 只有足够通用的文件系统助手才放到 `internal/state`

## 非目标

这一阶段明确不解决：

- 多任务
- 任务依赖
- 自动选择下一条命令
- 人工审核 UI
- 远程分发
- 可恢复的子进程继续执行

这些都应该留到这个状态机 runner 稳定之后再做。

## 成功标准

当下面这些都成立时，这一阶段就算成功：

- daemon 不再每个 tick 无脑重复执行命令
- 命令结果能驱动明确、可持久化的状态迁移
- `needs_review` 会阻止自动继续推进
- `approve`、`reject`、`resume` 行为可预测
- 被中断的 `running` 状态在重启后会被保守处理

做到这里，这个项目就从“可运行的 daemon 骨架”升级成了“带人工闸门的最小本地 runner”。
