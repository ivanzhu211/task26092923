package ticketmerge

// NewStore 创建一个空的工单归并库。
func NewStore() *Store {
	return &Store{
		groups:     make(map[int64]*group),
		owner:      make(map[int64]int64),
		messageIDs: make(map[int64]struct{}),
	}
}

// CreateTicket 创建一张独立成组的新工单，返回的工单编号即该组主编号。
// 工单编号从 1 开始单调递增。
func (s *Store) CreateTicket() int64 {
	s.nextTicketID++
	id := s.nextTicketID
	s.groups[id] = &group{id: id}
	s.owner[id] = id
	return id
}

// AppendMessage 向工单追加一条消息。
//
// ticketID 可以是任何旧编号（含已被并入其他组的编号），消息归属该编号
// 当前所在组。消息的原始来源固定记录为本次提交使用的 ticketID，此后无论
// 工单如何归并都不再改变；入参 msg.TicketID 会被该值覆盖。消息编号必须
// 全局唯一，乱序到达也可以正常追加。
func (s *Store) AppendMessage(ticketID int64, msg Message) error {
	g, ok := s.groupOf(ticketID)
	if !ok {
		return ErrTicketNotFound
	}
	if _, exists := s.messageIDs[msg.ID]; exists {
		return ErrDuplicateMessageID
	}
	msg.TicketID = ticketID
	s.messageIDs[msg.ID] = struct{}{}
	g.messages = append(g.messages, msg)
	return nil
}

// Resolve 返回工单编号当前所属组的主编号。
func (s *Store) Resolve(ticketID int64) (int64, error) {
	master, ok := s.owner[ticketID]
	if !ok {
		return 0, ErrTicketNotFound
	}
	return master, nil
}

// groupOf 返回编号当前所属的组。编号可以是主编号或任意历史别名。
func (s *Store) groupOf(ticketID int64) (*group, bool) {
	master, ok := s.owner[ticketID]
	if !ok {
		return nil, false
	}
	g, ok := s.groups[master]
	return g, ok
}
