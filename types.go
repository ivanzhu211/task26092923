// Package ticketmerge 是一个保存在进程内存中的客服工单归并与历史查询库。
//
// 同一客户可能通过邮件、聊天和电话反复描述同一问题，坐席需要把重复工单
// 合成一份完整历史。本库提供创建工单、追加消息、合并工单、分页读取历史
// 四种能力。工单编号一旦创建便永久可用：查询任何旧编号都会解析到其当前
// 所属组的主编号，并能读到整个组的完整历史。
package ticketmerge

import "errors"

// ErrTicketNotFound 表示访问了从未创建的工单编号。
var ErrTicketNotFound = errors.New("ticketmerge: ticket not found")

// ErrDuplicateMessageID 表示全局消息编号已经存在。消息编号全局唯一。
var ErrDuplicateMessageID = errors.New("ticketmerge: duplicate message id")

// ErrInvalidMerge 表示合并请求缺少来源编号，或分页参数非法。
var ErrInvalidMerge = errors.New("ticketmerge: invalid request")

// Message 是一条工单消息。
type Message struct {
	// ID 是全局唯一的整数消息编号。
	ID int64
	// TicketID 是消息最初提交时使用的工单编号，此后不再改变。
	TicketID int64
	// Timestamp 是整数时间戳，消息可能乱序到达。
	Timestamp int64
	// Body 是消息正文。
	Body string
}

// Cursor 是分页读取的续读位置。
//
// 零值表示从头读取；传入上一页 Page.Next 的值即可从下一条继续。
type Cursor struct {
	lastID        int64
	lastTimestamp int64
	hasLast       bool
}

// Page 是一页历史读取结果。
type Page struct {
	// Messages 按 (Timestamp, ID) 升序排列，最多包含请求的页大小条。
	Messages []Message
	// Next 是下一页的续读游标；当没有更多消息时为 nil。
	Next *Cursor
}

// group 是一组已经归并在一起的工单及其全部消息。
type group struct {
	// id 是本组当前的主编号，沿用某次合并中目标组合并前的主编号。
	id int64
	// messages 保存组内全部消息，追加时保持插入顺序，读取时再排序。
	messages []Message
}

// Store 是内存中的工单归并库。零值不可用，请通过 NewStore 创建。
type Store struct {
	// groups 是主编号 -> 组。
	groups map[int64]*group
	// owner 是任一工单编号（含历史别名）-> 当前所属组主编号。
	owner map[int64]int64
	// messageIDs 记录已使用的全局消息编号。
	messageIDs map[int64]struct{}
	// nextTicketID 是下一个新工单编号。
	nextTicketID int64
}
