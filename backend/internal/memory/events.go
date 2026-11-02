package memory

import "sync"

// Event 是推送给前端的实时事件。
type Event struct {
	Type        string `json:"type"`        // memory_updated / memory_reset
	Familiarity int    `json:"familiarity"` // 当前画像条数（含 0，前端需区分「0 件事」与「未收到」）
}

// hub 是进程内的发布/订阅中心：按 userID 隔离，支持多个订阅者。
type hub struct {
	mu   sync.Mutex
	subs map[string][]chan Event
}

func newHub() *hub {
	return &hub{subs: map[string][]chan Event{}}
}

// subscribe 订阅某用户的事件流，返回只读 channel 与取消函数。
func (h *hub) subscribe(userID string) (<-chan Event, func()) {
	ch := make(chan Event, 16)
	h.mu.Lock()
	h.subs[userID] = append(h.subs[userID], ch)
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		subs := h.subs[userID]
		for i, s := range subs {
			if s == ch {
				h.subs[userID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		if len(h.subs[userID]) == 0 {
			delete(h.subs, userID)
		}
		close(ch)
	}
	return ch, cancel
}

// publish 向某用户的所有订阅者广播事件（慢消费者直接丢弃，不阻塞）。
func (h *hub) publish(userID string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs[userID] {
		select {
		case ch <- ev:
		default:
		}
	}
}
