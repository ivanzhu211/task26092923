package ticketmerge

// CreateTicket 创建一张新工单，独立成组，返回的编号即主编号。
func (s *Store) CreateTicket() int {
	s.nextID++
	g := &group{primary: s.nextID, members: []int{s.nextID}}
	s.byTicket[s.nextID] = g
	return s.nextID
}

// AppendMessage 向 ticketID 当前所在组追加一条消息。
// 消息的 Source 记录为本次调用使用的 ticketID。
func (s *Store) AppendMessage(ticketID int, msg Message) error {
	g, ok := s.byTicket[ticketID]
	if !ok {
		return ErrUnknownTicket
	}
	msg.Source = ticketID
	g.messages = append(g.messages, msg)
	return nil
}

// Resolve 返回 ticketID 当前所属组的主编号。
func (s *Store) Resolve(ticketID int) (int, bool) {
	g, ok := s.byTicket[ticketID]
	if !ok {
		return 0, false
	}
	return g.primary, true
}
