package ticketmerge_test

import (
	"fmt"

	"ticketmerge"
)

// 演示创建工单、追加乱序消息、合并（含旧别名与同组重复）、逐页读取完整历史。
func ExampleStore() {
	s := ticketmerge.NewStore()

	email := s.CreateTicket()
	chat := s.CreateTicket()
	phone := s.CreateTicket()

	// 消息可能乱序到达；消息编号全局唯一。
	_ = s.AppendMessage(email, ticketmerge.Message{ID: 1, Timestamp: 100, Body: "无法登录"})
	_ = s.AppendMessage(chat, ticketmerge.Message{ID: 2, Timestamp: 200, Body: "一直转圈"})
	_ = s.AppendMessage(email, ticketmerge.Message{ID: 3, Timestamp: 50, Body: "更早的补充"})
	_ = s.AppendMessage(phone, ticketmerge.Message{ID: 4, Timestamp: 150, Body: "同一个问题"})

	// 邮件工单为主，合并聊天与电话；重复别名会被自动忽略。
	_, _ = s.Merge(email, []int64{chat, chat, phone})

	// 任意旧编号都解析到同一个主编号。
	for _, id := range []int64{email, chat, phone} {
		m, _ := s.Resolve(id)
		fmt.Printf("ticket %d -> master %d\n", id, m)
	}

	// 页大小为 2，用旧的电话编号逐页读取整个组。
	var cursor *ticketmerge.Cursor
	for pageNo := 1; ; pageNo++ {
		page, _ := s.History(phone, cursor, 2)
		for _, msg := range page.Messages {
			fmt.Printf("p%d: id=%d ts=%d source=%d %q\n", pageNo, msg.ID, msg.Timestamp, msg.TicketID, msg.Body)
		}
		if page.Next == nil {
			break
		}
		cursor = page.Next
	}
	// Output:
	// ticket 1 -> master 1
	// ticket 2 -> master 1
	// ticket 3 -> master 1
	// p1: id=3 ts=50 source=1 "更早的补充"
	// p1: id=1 ts=100 source=1 "无法登录"
	// p2: id=4 ts=150 source=3 "同一个问题"
	// p2: id=2 ts=200 source=2 "一直转圈"
}
