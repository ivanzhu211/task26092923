package ticketmerge

import "sort"

// History 分页读取 ticketID 当前所在组的历史。
// after 为 nil 时从头读取，否则从该游标之后继续。
// 消息按 (Timestamp, ID) 升序返回。
func (s *Store) History(ticketID int, limit int, after *Cursor) (Page, error) {
	if limit <= 0 {
		return Page{}, ErrInvalidLimit
	}
	g, ok := s.byTicket[ticketID]
	if !ok {
		return Page{}, ErrUnknownTicket
	}
	sorted := make([]Message, len(g.messages))
	copy(sorted, g.messages)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Timestamp != sorted[j].Timestamp {
			return sorted[i].Timestamp < sorted[j].Timestamp
		}
		return sorted[i].ID < sorted[j].ID
	})
	start := 0
	if after != nil {
		start = sort.Search(len(sorted), func(i int) bool {
			m := sorted[i]
			return m.Timestamp > after.Timestamp ||
				(m.Timestamp == after.Timestamp && m.ID > after.MessageID)
		})
	}
	end := start + limit
	if end > len(sorted) {
		end = len(sorted)
	}
	page := Page{
		Primary:  g.primary,
		Messages: sorted[start:end],
		HasMore:  end < len(sorted),
	}
	if len(page.Messages) > 0 {
		last := page.Messages[len(page.Messages)-1]
		page.Next = Cursor{Timestamp: last.Timestamp, MessageID: last.ID}
	}
	return page, nil
}
