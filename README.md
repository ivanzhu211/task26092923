# ticketmerge

进程内存中的客服工单归并与历史查询库（Go），由上层业务直接调用，无外部依赖。

## 能力

- **创建工单** `CreateTicket() int`：新工单独立成组，返回的编号即主编号。
- **追加消息** `AppendMessage(ticketID, Message) error`：消息归属 `ticketID` 当前所在组；
  `Message.Source` 由库记录为本次调用使用的编号，此后不变。消息可乱序到达。
- **合并工单** `Merge(target, sources...) error`：编号先解释为其当前所属组，同组只处理一次，
  已在目标组的来源不产生变化；合并后主编号保持目标组在合并前的主编号。
  所有旧编号永久有效，查询任何旧编号都返回当前主编号及整组历史。
- **分页读取历史** `History(ticketID, limit, after) (Page, error)`：按 `(Timestamp, ID)` 升序。
  `after` 传 `nil` 从头读，否则传上一页的 `Page.Next` 继续；`Page.HasMore` 为 false 时读完。
  完整翻页恰好覆盖组内全部消息，无重复无遗漏。
- **查询主编号** `Resolve(ticketID) (int, bool)`：返回编号当前所属组的主编号。

## 示例

```go
s := ticketmerge.NewStore()
a := s.CreateTicket() // 1
b := s.CreateTicket() // 2

s.AppendMessage(a, ticketmerge.Message{ID: 1, Timestamp: 100, Body: "邮件描述问题"})
s.AppendMessage(b, ticketmerge.Message{ID: 2, Timestamp: 90, Body: "聊天重复反馈"})

s.Merge(a, b) // b 并入 a，主编号为 a

// 向旧编号 b 追加，消息归属 a 组，Source 仍记为 b
s.AppendMessage(b, ticketmerge.Message{ID: 3, Timestamp: 110, Body: "电话补充"})

// 分页读取（用任意旧编号均可）
var after *ticketmerge.Cursor
for {
    page, _ := s.History(b, 2, after)
    for _, m := range page.Messages {
        fmt.Println(m.Timestamp, m.ID, m.Source, m.Body)
    }
    if !page.HasMore {
        break
    }
    c := page.Next
    after = &c
}
```

## 约束

- 仅供单 goroutine 顺序调用；一次连续翻页期间不应发生追加或合并。
- 使用从未创建的编号会返回 `ErrUnknownTicket`；`limit <= 0` 返回 `ErrInvalidLimit`。

## 运行测试

```sh
go test ./...
```

测试覆盖：多次更换主编号后向最早编号追加迟到消息；同时间戳消息跨页与同组别名重复合并；
混合操作下所有旧编号指向最终主编号、消息来源与正文保持不变、完整分页严格有序且无重复遗漏。
