package ticketmerge

import (
	"fmt"
	"testing"
)

// drain 从 ticketID 当前所在组完整翻页，校验每页主编号一致且严格有序，
// 返回全部消息。

// wantMsg 记录一条消息的期望正文与来源编号。
type wantMsg struct {
	body   string
	source int
}

func drain(t *testing.T, s *Store, ticketID, pageSize int) []Message {
	t.Helper()
	var all []Message
	var after *Cursor
	wantPrimary, ok := s.Resolve(ticketID)
	if !ok {
		t.Fatalf("Resolve(%d) failed", ticketID)
	}
	for {
		page, err := s.History(ticketID, pageSize, after)
		if err != nil {
			t.Fatalf("History(%d): %v", ticketID, err)
		}
		if page.Primary != wantPrimary {
			t.Fatalf("page primary = %d, want %d", page.Primary, wantPrimary)
		}
		if len(page.Messages) == 0 && page.HasMore {
			t.Fatalf("empty page with HasMore=true")
		}
		for _, m := range page.Messages {
			if len(all) > 0 {
				prev := all[len(all)-1]
				if m.Timestamp < prev.Timestamp ||
					(m.Timestamp == prev.Timestamp && m.ID <= prev.ID) {
					t.Fatalf("order violation: %+v after %+v", m, prev)
				}
			}
			all = append(all, m)
		}
		if !page.HasMore {
			break
		}
		c := page.Next
		after = &c
	}
	return all
}

// checkMessages 校验消息集合与期望完全一致：编号、正文、来源编号。
func checkMessages(t *testing.T, got []Message, want map[int64]wantMsg) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d messages, want %d", len(got), len(want))
	}
	seen := make(map[int64]bool)
	for _, m := range got {
		if seen[m.ID] {
			t.Fatalf("duplicate message id %d", m.ID)
		}
		seen[m.ID] = true
		w, ok := want[m.ID]
		if !ok {
			t.Fatalf("unexpected message id %d", m.ID)
		}
		if m.Body != w.body {
			t.Fatalf("message %d body = %q, want %q", m.ID, m.Body, w.body)
		}
		if m.Source != w.source {
			t.Fatalf("message %d source = %d, want %d", m.ID, w.source, w.source)
		}
	}
}

// 场景一：多次更换主编号后，向最早编号追加迟到消息。
func TestLateMessageToEarliestIDAfterPrimaryChanges(t *testing.T) {
	s := NewStore()
	a := s.CreateTicket() // 1
	b := s.CreateTicket() // 2
	c := s.CreateTicket() // 3
	d := s.CreateTicket() // 4

	mustAppend(t, s, a, Message{ID: 1, Timestamp: 10, Body: "a1"})
	mustAppend(t, s, b, Message{ID: 2, Timestamp: 20, Body: "b1"})
	mustAppend(t, s, c, Message{ID: 3, Timestamp: 30, Body: "c1"})

	// 第一次合并：b 并入 a，主编号为 a。
	mustMerge(t, s, a, b)
	// 第二次合并：a 组并入 c，主编号为 c。
	mustMerge(t, s, c, a)
	// 第三次合并：c 组并入 d，主编号为 d。
	mustMerge(t, s, d, c)

	// 向最早编号 a 追加迟到消息，归属当前组（主编号 d）。
	mustAppend(t, s, a, Message{ID: 4, Timestamp: 5, Body: "late"})
	// 向中间主编号 c 追加，同样归属当前组。
	mustAppend(t, s, c, Message{ID: 5, Timestamp: 40, Body: "via-c"})

	// 所有旧编号都必须指向最终主编号 d，不能停在中间主编号上。
	for _, id := range []int{a, b, c, d} {
		got, ok := s.Resolve(id)
		if !ok || got != d {
			t.Fatalf("Resolve(%d) = %d, %v; want %d, true", id, got, ok, d)
		}
	}

	want := map[int64]wantMsg{
		1: {"a1", a},
		2: {"b1", b},
		3: {"c1", c},
		4: {"late", a},
		5: {"via-c", c},
	}
	// 用每个旧编号读取，历史必须一致且完整。
	for _, id := range []int{a, b, c, d} {
		all := drain(t, s, id, 2)
		checkMessages(t, all, want)
	}
	// 乱序到达：迟到消息时间戳最小，应排在最前。
	first := drain(t, s, a, 100)[0]
	if first.ID != 4 {
		t.Fatalf("first message id = %d, want 4 (late message)", first.ID)
	}
}

// 场景二：时间戳相同的消息跨越多页，同组别名重复参与合并。
func TestSameTimestampAcrossPagesAndDuplicateAliases(t *testing.T) {
	s := NewStore()
	x := s.CreateTicket() // 1
	y := s.CreateTicket() // 2
	z := s.CreateTicket() // 3

	// 同一组以不同别名重复出现在来源列表中，只处理一次。
	mustMerge(t, s, x, y, y, x, y)
	// z 先与 y 合并（y 已属 x 组），再用旧编号 y 作为目标合并 z 的别名。
	mustMerge(t, s, y, z, z, x)

	// 10 条消息共享同一时间戳，跨越多页。
	const n = 10
	want := make(map[int64]wantMsg)
	for i := 0; i < n; i++ {
		id := int64(100 + i)
		src := []int{x, y, z}[i%3]
		body := fmt.Sprintf("m%d", i)
		mustAppend(t, s, src, Message{ID: id, Timestamp: 42, Body: body})
		want[id] = wantMsg{body, src}
	}

	// 页大小为 3，同时间戳消息跨 4 页，按消息编号升序衔接。
	all := drain(t, s, z, 3)
	checkMessages(t, all, want)
	for i, m := range all {
		if m.ID != int64(100+i) {
			t.Fatalf("position %d: id = %d, want %d", i, m.ID, 100+i)
		}
	}

	// 所有别名解析到 x（目标组在合并前的主编号）。
	for _, id := range []int{x, y, z} {
		got, _ := s.Resolve(id)
		if got != x {
			t.Fatalf("Resolve(%d) = %d, want %d", id, got, x)
		}
	}
}

// 场景三：混合操作，乱序消息、多次合并、旧编号追加、完整分页覆盖校验。
func TestMixedOperations(t *testing.T) {
	s := NewStore()
	t1 := s.CreateTicket()
	t2 := s.CreateTicket()
	t3 := s.CreateTicket()
	t4 := s.CreateTicket()
	t5 := s.CreateTicket()

	want := make(map[int64]wantMsg)
	add := func(ticket int, id, ts int64, body string) {
		mustAppend(t, s, ticket, Message{ID: id, Timestamp: ts, Body: body})
		want[id] = wantMsg{body, ticket}
	}

	// 乱序追加。
	add(t1, 5, 50, "t1-c")
	add(t1, 1, 10, "t1-a")
	add(t2, 3, 30, "t2-a")
	add(t3, 4, 40, "t3-a")
	add(t1, 2, 20, "t1-b")

	// 合并后向旧编号追加。
	mustMerge(t, s, t2, t1)
	add(t1, 6, 60, "t1-late") // 归属 t2 组
	mustMerge(t, s, t4, t3)
	// 两个已合并的组再次合并：t2 组并入 t4 组。
	mustMerge(t, s, t4, t2)
	add(t3, 7, 70, "t3-late") // 归属 t4 组
	// t5 保持独立。
	add(t5, 8, 80, "t5-a")

	// 大组：t1..t4 全部指向 t4。
	for _, id := range []int{t1, t2, t3, t4} {
		got, _ := s.Resolve(id)
		if got != t4 {
			t.Fatalf("Resolve(%d) = %d, want %d", id, got, t4)
		}
	}
	if got, _ := s.Resolve(t5); got != t5 {
		t.Fatalf("Resolve(%d) = %d, want %d", t5, got, t5)
	}

	// 完整翻页恰好覆盖大组全部 7 条消息。
	bigWant := make(map[int64]wantMsg)
	for id, v := range want {
		if id != 8 {
			bigWant[id] = v
		}
	}
	for _, id := range []int{t1, t2, t3, t4} {
		checkMessages(t, drain(t, s, id, 3), bigWant)
	}
	// 独立组只有自己的消息。
	solo := drain(t, s, t5, 10)
	if len(solo) != 1 || solo[0].ID != 8 || solo[0].Source != t5 {
		t.Fatalf("t5 history = %+v", solo)
	}
	// 未创建编号报错。
	if _, err := s.History(999, 10, nil); err != ErrUnknownTicket {
		t.Fatalf("History(999) err = %v", err)
	}
	if err := s.AppendMessage(999, Message{ID: 9}); err != ErrUnknownTicket {
		t.Fatalf("AppendMessage(999) err = %v", err)
	}
	if err := s.Merge(999, t1); err != ErrUnknownTicket {
		t.Fatalf("Merge(999) err = %v", err)
	}
}

func mustAppend(t *testing.T, s *Store, ticket int, msg Message) {
	t.Helper()
	if err := s.AppendMessage(ticket, msg); err != nil {
		t.Fatalf("AppendMessage(%d, %+v): %v", ticket, msg, err)
	}
}

func mustMerge(t *testing.T, s *Store, target int, sources ...int) {
	t.Helper()
	if err := s.Merge(target, sources...); err != nil {
		t.Fatalf("Merge(%d, %v): %v", target, sources, err)
	}
}
