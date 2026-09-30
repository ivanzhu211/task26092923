package ticketmerge

// Merge 把若干来源工单所属的组并入目标工单所属的组，并返回合并后的主编号。
//
// 编号先解释为其当前所属组：
//   - 目标编号、来源编号都可以是主编号或任意历史别名；
//   - 涉及同一组的多个编号只处理一次；
//   - 来源已属于目标组时不产生变化；
//   - 目标编号与来源编号必须都是已创建的工单。
//
// 合并后所有涉及的组成为一个组，主编号采用目标组在本次合并前的主编号；
// 来源组（含其全部历史别名、消息）整体并入，消息不丢失、不复制、不改写。
func (s *Store) Merge(targetID int64, sourceIDs []int64) (int64, error) {
	if len(sourceIDs) == 0 {
		return 0, ErrInvalidMerge
	}
	targetMaster, ok := s.owner[targetID]
	if !ok {
		return 0, ErrTicketNotFound
	}

	// seen 记录本次合并已经处理过的组（含目标组），保证同组只处理一次。
	seen := map[int64]struct{}{targetMaster: {}}

	targetGroup := s.groups[targetMaster]
	for _, sourceID := range sourceIDs {
		sourceMaster, ok := s.owner[sourceID]
		if !ok {
			return 0, ErrTicketNotFound
		}
		if _, dup := seen[sourceMaster]; dup {
			// 同一组的别名重复出现，或来源已在目标组中：跳过。
			continue
		}
		seen[sourceMaster] = struct{}{}

		sourceGroup := s.groups[sourceMaster]
		targetGroup.messages = append(targetGroup.messages, sourceGroup.messages...)

		// 该组的全部编号（主编号与所有历史别名）改指目标主编号。
		for alias, master := range s.owner {
			if master == sourceMaster {
				s.owner[alias] = targetMaster
			}
		}
		delete(s.groups, sourceMaster)
	}
	return targetMaster, nil
}
