# ticketmerge — 客服工单归并与历史查询库

纯 Go、零依赖、保存在**进程内存**中的工单库，由上层业务直接调用。同一客户
通过邮件、聊天、电话反复描述同一问题时，坐席可把重复工单合成一份完整历史；
已合并的工单可以继续收消息、也可以再次被并入另一张工单，所有旧编号持续可用。

## 能力

- `NewStore()` / `CreateTicket()`：新工单独立成组，自身编号即主编号（编号从 1 起递增）。
- `AppendMessage(ticketID, msg)`：向工单追加消息；可用任意旧编号，消息归属其**当前所在组**。
- `Merge(targetID, sourceIDs)`：把若干来源所属的组合并进目标所属的组。
- `History(ticketID, cursor, limit)`：按时间戳 + 消息编号升序分页读取整组历史。
- `Resolve(ticketID)`：返回编号当前所属组的主编号。

## 归并语义

- 所有编号先解释为其**当前所属组**：目标和来源都可以是主编号或历史别名。
- 同一次合并中同一组只处理一次；来源已属于目标组时不产生变化、不复制消息。
- 合并后主编号采用**目标组在本次合并前的主编号**；来源组的全部历史别名改指到它。
- 组被再次合并时，所有更早的编号继续跟随，不会停在某个中间主编号上。

## 消息与排序

- 每条消息带全局唯一整数编号 `ID`、整数 `Timestamp`、正文 `Body`；消息允许乱序到达。
- 消息的原始来源固定为**最初提交时使用的工单编号**（`Message.TicketID`），归并不改写它。
- 历史统一按 `(Timestamp, ID)` 升序。游标为键集（keyset）分页：
  - `cursor == nil` 从头读；之后传入上一页 `Page.Next` 继续；
  - 同时间戳消息被切到不同页时，游标同时携带时间戳与消息编号，严格续读；
  - 完整翻页恰好覆盖组内全部消息，**无重复、无遗漏、全程有序**；`Page.Next == nil` 表示读完。

## 错误

- `ErrTicketNotFound`：编号从未创建。
- `ErrDuplicateMessageID`：消息编号已存在。
- `ErrInvalidMerge`：来源列表为空，或 `limit <= 0`。

## 快速使用

```go
s := ticketmerge.NewStore()
email := s.CreateTicket()
chat  := s.CreateTicket()

_ = s.AppendMessage(email, ticketmerge.Message{ID: 1, Timestamp: 100, Body: "无法登录"})
_ = s.AppendMessage(chat,  ticketmerge.Message{ID: 2, Timestamp: 200, Body: "一直转圈"})

_, _ = s.Merge(email, []int64{chat})        // 主编号为 email
master, _ := s.Resolve(chat)                // chat 与 email 都解析到 email

var cursor *ticketmerge.Cursor
for {
    page, err := s.History(chat, cursor, 50) // 用旧编号也能读到整组
    if err != nil { break }
    for _, m := range page.Messages { /* 处理 m */ }
    if page.Next == nil { break }
    cursor = page.Next
}
```

可运行的完整示例见 `example_test.go` 中的 `ExampleStore`。

## 运行测试

```bash
go test ./... -v
```

测试覆盖：

- 多次更换主编号后，向**最早编号**追加迟到消息，且所有旧编号指向最终主编号；
- 相同时间戳消息跨越多页、同组别名重复参与合并、冗余再次合并；
- 随机乱序消息以页大小 1 逐页穷举，严格有序且无重复/遗漏；
- 经旧别名发起合并、合并后继续向旧编号追加消息，正文与原始来源不变；
- 各类错误路径。

## 约束

- 数据只存内存，进程退出即丢失；不依赖数据库、缓存或任何监听端口的服务。
- 所有方法**顺序调用**，非并发安全；一次连续翻页期间上层不会追加或合并。
