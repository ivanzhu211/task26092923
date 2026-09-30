// Package ticketmerge 提供进程内存中的客服工单归并与历史查询能力。
package ticketmerge

import "errors"

// ErrUnknownTicket 表示使用了从未创建过的工单编号。
var ErrUnknownTicket = errors.New("ticketmerge: unknown ticket id")

// ErrInvalidLimit 表示分页大小不合法。
var ErrInvalidLimit = errors.New("ticketmerge: page limit must be positive")

// Message 是一条工单消息。
// ID 全局唯一；Timestamp 为整数时间戳；Body 为正文。
// Source 为最初提交消息时使用的工单编号，由 Store 在追加时记录，此后不变。
type Message struct {
	ID        int64
	Timestamp int64
	Body      string
	Source    int
}

// Cursor 是排序键游标，标识已读到的最后一条消息。
type Cursor struct {
	Timestamp int64
	MessageID int64
}

// Page 是一次历史读取的结果。
type Page struct {
	Primary  int       // 该编号当前所属组的主编号
	Messages []Message // 本页消息，按 (Timestamp, ID) 升序
	Next     Cursor    // 下一页起始游标（HasMore 为 true 时有效）
	HasMore  bool      // 是否还有后续页
}

// group 是一个工单组：若干工单编号共享同一主编号与同一份消息历史。
type group struct {
	primary  int
	members  []int
	messages []Message
}

// Store 保存所有工单与消息，仅供单 goroutine 顺序调用。
type Store struct {
	nextID   int
	byTicket map[int]*group
}

// NewStore 创建一个空的 Store。
func NewStore() *Store {
	return &Store{byTicket: make(map[int]*group)}
}
