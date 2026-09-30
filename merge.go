package ticketmerge

// Merge 将 sources 各自当前所属的组并入 target 当前所属的组。
// 同一组重复出现时只处理一次；已在目标组中的来源不产生变化。
// 合并后主编号保持目标组在本次合并前的主编号。
func (s *Store) Merge(target int, sources ...int) error {
	tg, ok := s.byTicket[target]
	if !ok {
		return ErrUnknownTicket
	}
	seen := map[*group]bool{tg: true}
	var absorb []*group
	for _, src := range sources {
		sg, ok := s.byTicket[src]
		if !ok {
			return ErrUnknownTicket
		}
		if seen[sg] {
			continue
		}
		seen[sg] = true
		absorb = append(absorb, sg)
	}
	for _, sg := range absorb {
		tg.messages = append(tg.messages, sg.messages...)
		for _, id := range sg.members {
			s.byTicket[id] = tg
		}
		tg.members = append(tg.members, sg.members...)
	}
	return nil
}
