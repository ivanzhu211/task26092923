package ticketmerge

import (
	"fmt"
	"math/rand"
	"testing"
)

// readAll 用给定页大小从头翻到尾，返回全部消息。
func readAll(t *testing.T, s *Store, ticketID int64, limit int) []Message {
	t.Helper()
	var out []Message
	cursor := (*Cursor)(nil)
	pages := 0
	for {
		page, err := s.History(ticketID, cursor, limit)
		if err != nil {
			t.Fatalf("history: %v", err)
		}
		out = append(out, page.Messages...)
		pages++
		if pages > 10000 {
			t.Fatal("pagination did not terminate")
		}
		if page.Next == nil {
			break
		}
		cursor = page.Next
	}
	return out
}

func assertOrderedUnique(t *testing.T, msgs []Message) {
	t.Helper()
	seen := map[int64]struct{}{}
	for i, m := range msgs {
		if i > 0 {
			prev := msgs[i-1]
			if prev.Timestamp > m.Timestamp ||
				(prev.Timestamp == m.Timestamp && prev.ID >= m.ID) {
				t.Fatalf("order violated at %d: {%d ts=%d} before {%d ts=%d}",
					i, prev.ID, prev.Timestamp, m.ID, m.Timestamp)
			}
		}
		if _, dup := seen[m.ID]; dup {
			t.Fatalf("duplicate message id %d during pagination", m.ID)
		}
		seen[m.ID] = struct{}{}
	}
}

// 场景一：多次更换主编号后，向最早编号追加迟到消息。
func TestMasterChainsAndLateMessageToEarliestID(t *testing.T) {
	s := NewStore()
	t1 := s.CreateTicket() // 1
	t2 := s.CreateTicket() // 2
	t3 := s.CreateTicket() // 3
	t4 := s.CreateTicket() // 4

	mustMerge := func(target int64, sources ...int64) {
		t.Helper()
		if _, err := s.Merge(target, sources); err != nil {
			t.Fatalf("merge %v -> %d: %v", sources, target, err)
		}
	}

	mustMerge(t2, t1) // 主编号 2
	mustMerge(t3, t2) // 主编号 3，旧编号 1、2 跟随
	mustMerge(t4, t3) // 主编号 4，更早编号 1、2、3 也要跟随

	for _, id := range []int64{t1, t2, t3, t4} {
		m, err := s.Resolve(id)
		if err != nil || m != t4 {
			t.Fatalf("id %d should follow chain to master %d, got %d err=%v", id, t4, m, err)
		}
	}

	// 合并过程中在各阶段加入消息。
	_ = s.AppendMessage(t1, Message{ID: 101, Timestamp: 100, Body: "from-1"})
	_ = s.AppendMessage(t2, Message{ID: 102, Timestamp: 200, Body: "from-2"})
	_ = s.AppendMessage(t3, Message{ID: 103, Timestamp: 300, Body: "from-3"})

	// 向最早编号 t1 追加一条迟到消息，时间戳反而最早，且通过最旧的别名访问。
	if err := s.AppendMessage(t1, Message{ID: 104, Timestamp: 5, Body: "late-to-1"}); err != nil {
		t.Fatalf("late append to earliest id: %v", err)
	}

	msgs := readAll(t, s, t1, 2) // 用最早编号翻全部历史，页大小 2
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	assertOrderedUnique(t, msgs)
	if msgs[0].ID != 104 || msgs[0].TicketID != t1 {
		t.Fatalf("late message should be first with original source %d, got %+v", t1, msgs[0])
	}
	wantSource := map[int64]int64{101: t1, 102: t2, 103: t3, 104: t1}
	for _, m := range msgs {
		if m.TicketID != wantSource[m.ID] {
			t.Fatalf("message %d original source changed: got %d want %d", m.ID, m.TicketID, wantSource[m.ID])
		}
	}
}

// 场景二：相同时间戳的消息跨越多页；同组别名重复参与合并。
func TestSameTimestampAcrossPagesAndDuplicateAliases(t *testing.T) {
	s := NewStore()
	a := s.CreateTicket()
	b := s.CreateTicket()
	c := s.CreateTicket()

	// 三组消息全部同一时间戳，仅靠消息编号区分次序，共 6 条。
	for i, ticket := range []int64{a, b, c} {
		for j := 0; j < 2; j++ {
			id := int64(200 + i*10 + j)
			if err := s.AppendMessage(ticket, Message{ID: id, Timestamp: 50, Body: fmt.Sprintf("t%d-%d", ticket, j)}); err != nil {
				t.Fatal(err)
			}
		}
	}

	// 来源里 b 重复、目标别名 a 自身、c 各出现：每组只应处理一次。
	master, err := s.Merge(a, []int64{b, b, a, c, b})
	if err != nil || master != a {
		t.Fatalf("merge dedup failed: master=%d err=%v", master, err)
	}
	for _, id := range []int64{a, b, c} {
		if m, _ := s.Resolve(id); m != a {
			t.Fatalf("id %d should resolve to %d", id, a)
		}
	}

	// 页大小 3：同一时间戳必然被切到不同页，游标必须携带时间戳正确续读。
	msgs := readAll(t, s, c, 3)
	if len(msgs) != 6 {
		t.Fatalf("duplicate aliases must not duplicate messages: got %d, want 6", len(msgs))
	}
	assertOrderedUnique(t, msgs)
	wantIDs := []int64{200, 201, 210, 211, 220, 221}
	for i, want := range wantIDs {
		if msgs[i].ID != want {
			t.Fatalf("at %d want id %d got %d (same-timestamp tie by id)", i, want, msgs[i].ID)
		}
	}

	// 全部已是同组，再次合并不产生变化、不复制消息。
	if _, err := s.Merge(a, []int64{b, c}); err != nil {
		t.Fatal(err)
	}
	msgs = readAll(t, s, a, 3)
	if len(msgs) != 6 {
		t.Fatalf("redundant merge changed message count: got %d", len(msgs))
	}
}

// 场景三：随机乱序 + 逐页穷举，严格有序、无重复无遗漏。
func TestExhaustivePaginationNoLossNoDup(t *testing.T) {
	s := NewStore()
	g1t1 := s.CreateTicket()
	g1t2 := s.CreateTicket()
	g2t1 := s.CreateTicket()

	rng := rand.New(rand.NewSource(42))
	total := 25
	want := map[int64]Message{}
	add := func(ticket int64, id int64) {
		m := Message{ID: id, Timestamp: rng.Int63n(5), Body: fmt.Sprintf("body-%d", id)}
		if err := s.AppendMessage(ticket, m); err != nil {
			t.Fatal(err)
		}
		m.TicketID = ticket
		want[id] = m
	}
	for i := int64(0); i < int64(total); i++ {
		switch i % 3 {
		case 0:
			add(g1t1, 1000+i)
		case 1:
			add(g1t2, 1000+i)
		default:
			add(g2t1, 1000+i)
		}
	}

	if _, err := s.Merge(g1t1, []int64{g2t1}); err != nil { // 两组历史本就同组的 g1t2 也验证一次
		t.Fatal(err)
	}
	if _, err := s.Merge(g1t1, []int64{g1t2}); err != nil {
		t.Fatal(err)
	}

	// 页大小为 1，最苛刻地检验游标边界。
	msgs := readAll(t, s, g2t1, 1)
	if len(msgs) != total {
		t.Fatalf("expected exactly %d messages across pages, got %d", total, len(msgs))
	}
	assertOrderedUnique(t, msgs)
	for _, m := range msgs {
		w, ok := want[m.ID]
		if !ok {
			t.Fatalf("unexpected message id %d", m.ID)
		}
		if m.Body != w.Body || m.TicketID != w.TicketID || m.Timestamp != w.Timestamp {
			t.Fatalf("message %d mutated after merges: got %+v want %+v", m.ID, m, w)
		}
		delete(want, m.ID)
	}
	if len(want) != 0 {
		t.Fatalf("%d messages missing from pagination: %v", len(want), want)
	}
}

// 场景四：组再次被并入另一张已有组的工单，目标本身也是旧别名；全程混合操作。
func TestRemergeGroupsViaAliases(t *testing.T) {
	s := NewStore()
	a := s.CreateTicket()
	b := s.CreateTicket()
	c := s.CreateTicket()
	d := s.CreateTicket()

	// 先形成两组：组主 a（含 b），组主 c。
	if _, err := s.Merge(a, []int64{b}); err != nil {
		t.Fatal(err)
	}
	_ = s.AppendMessage(a, Message{ID: 1, Timestamp: 1, Body: "a"})
	_ = s.AppendMessage(b, Message{ID: 2, Timestamp: 2, Body: "b"})
	_ = s.AppendMessage(c, Message{ID: 3, Timestamp: 3, Body: "c"})
	_ = s.AppendMessage(d, Message{ID: 4, Timestamp: 4, Body: "d"})

	// 用别名 b 作为目标、把独立组 c、d 并入；目标组主编号仍是合并前的 a。
	master, err := s.Merge(b, []int64{c, d})
	if err != nil || master != a {
		t.Fatalf("merge via alias should resolve target group and keep its pre-merge master %d: got %d err=%v", a, master, err)
	}
	for _, id := range []int64{a, b, c, d} {
		if m, _ := s.Resolve(id); m != a {
			t.Fatalf("id %d should finally resolve to %d", id, a)
		}
	}

	// 合并后继续向旧编号追加消息。
	_ = s.AppendMessage(d, Message{ID: 5, Timestamp: 0, Body: "after-merge-d"})
	msgs := readAll(t, s, b, 2)
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}
	assertOrderedUnique(t, msgs)
	if msgs[0].ID != 5 || msgs[0].TicketID != d {
		t.Fatalf("post-merge message to old id misplaced or source lost: %+v", msgs[0])
	}
}
