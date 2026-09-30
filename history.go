package ticketmerge

import "sort"

// History 分页读取工单所属组的历史。
//
// ticketID 可以是主编号或任意历史别名，读取的都是整个组的历史。历史统一
// 按时间戳升序、同时间戳按消息编号升序排列。cursor 为 nil（或零值）表示
// 从头读取；之后传入上一页 Page.Next 即从上一页末条之后继续。limit 是单
// 页最大条数，必须大于 0。
//
// 完整翻页恰好覆盖组内全部消息：每条消息出现一次，不重复、不遗漏，且全程
// 严格有序。
func (s *Store) History(ticketID int64, cursor *Cursor, limit int) (*Page, error) {
	if limit <= 0 {
		return nil, ErrInvalidMerge
	}
	g, ok := s.groupOf(ticketID)
	if !ok {
		return nil, ErrTicketNotFound
	}

	ordered := make([]Message, len(g.messages))
	copy(ordered, g.messages)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Timestamp != ordered[j].Timestamp {
			return ordered[i].Timestamp < ordered[j].Timestamp
		}
		return ordered[i].ID < ordered[j].ID
	})

	start := 0
	if cursor != nil && cursor.hasLast {
		// 键集定位：第一条严格大于上一页末条排序键 (Timestamp, ID) 的消息。
		start = sort.Search(len(ordered), func(i int) bool {
			if ordered[i].Timestamp != cursor.lastTimestamp {
				return ordered[i].Timestamp > cursor.lastTimestamp
			}
			return ordered[i].ID > cursor.lastID
		})
	}

	page := &Page{}
	end := start + limit
	if end > len(ordered) {
		end = len(ordered)
	}
	if start < end {
		page.Messages = append(page.Messages, ordered[start:end]...)
	}
	if end < len(ordered) {
		last := ordered[end-1]
		page.Next = &Cursor{
			lastID:        last.ID,
			lastTimestamp: last.Timestamp,
			hasLast:       true,
		}
	}
	return page, nil
}
