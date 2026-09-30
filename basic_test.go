package ticketmerge

import (
	"errors"
	"testing"
)

func TestCreateTicketIsIndependentGroup(t *testing.T) {
	s := NewStore()
	a := s.CreateTicket()
	b := s.CreateTicket()
	if a == 0 || b == 0 || a == b {
		t.Fatalf("ticket ids must be distinct and non-zero, got %d %d", a, b)
	}
	for _, id := range []int64{a, b} {
		master, err := s.Resolve(id)
		if err != nil || master != id {
			t.Fatalf("ticket %d should be its own master, got master=%d err=%v", id, master, err)
		}
	}
}

func TestAppendAndHistoryOrdering(t *testing.T) {
	s := NewStore()
	tk := s.CreateTicket()
	// 乱序追加。
	msgs := []Message{
		{ID: 3, Timestamp: 30, Body: "c"},
		{ID: 1, Timestamp: 10, Body: "a"},
		{ID: 2, Timestamp: 20, Body: "b"},
	}
	for _, m := range msgs {
		if err := s.AppendMessage(tk, m); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	page, err := s.History(tk, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Next != nil {
		t.Fatal("single page should have no next cursor")
	}
	gotIDs := []int64{page.Messages[0].ID, page.Messages[1].ID, page.Messages[2].ID}
	if len(page.Messages) != 3 || gotIDs[0] != 1 || gotIDs[1] != 2 || gotIDs[2] != 3 {
		t.Fatalf("history not ordered by (timestamp,id): %+v", gotIDs)
	}
	for _, m := range page.Messages {
		if m.TicketID != tk {
			t.Fatalf("original source should be %d, got %d", tk, m.TicketID)
		}
	}
}

func TestBasicMerge(t *testing.T) {
	s := NewStore()
	a := s.CreateTicket()
	b := s.CreateTicket()
	_ = s.AppendMessage(a, Message{ID: 1, Timestamp: 1, Body: "a1"})
	_ = s.AppendMessage(b, Message{ID: 2, Timestamp: 2, Body: "b1"})

	master, err := s.Merge(a, []int64{b})
	if err != nil || master != a {
		t.Fatalf("merge should keep target as master, got %d err=%v", master, err)
	}
	for _, id := range []int64{a, b} {
		m, err := s.Resolve(id)
		if err != nil || m != a {
			t.Fatalf("id %d should resolve to %d, got %d err=%v", id, a, m, err)
		}
	}
	page, _ := s.History(b, nil, 10) // 用旧编号也能读到整组
	if len(page.Messages) != 2 {
		t.Fatalf("merged history should contain 2 messages, got %d", len(page.Messages))
	}
}

func TestErrors(t *testing.T) {
	s := NewStore()
	if err := s.AppendMessage(999, Message{ID: 1}); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
	if _, err := s.Resolve(999); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
	if _, err := s.Merge(999, []int64{1}); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
	tk := s.CreateTicket()
	if err := s.AppendMessage(tk, Message{ID: 7}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendMessage(tk, Message{ID: 7}); !errors.Is(err, ErrDuplicateMessageID) {
		t.Fatalf("expected ErrDuplicateMessageID, got %v", err)
	}
	if _, err := s.Merge(tk, nil); !errors.Is(err, ErrInvalidMerge) {
		t.Fatalf("expected ErrInvalidMerge, got %v", err)
	}
	if _, err := s.History(tk, nil, 0); !errors.Is(err, ErrInvalidMerge) {
		t.Fatalf("expected ErrInvalidMerge, got %v", err)
	}
}
