package dispatcher

import "github.com/devagame/due/v2/core/endpoint"

type Event struct {
	abstract
	event int // 事件ID
}

func newEvent(event int) *Event {
	return &Event{
		event:    event,
		abstract: newAbstract(),
	}
}

// Event 获取事件
func (e *Event) Event() int {
	return e.event
}

// VisitEndpoints 迭代服务端口
func (e *Event) VisitEndpoints(fn func(insID string, ep *endpoint.Endpoint) bool) {
	for _, se := range e.endpoints1 {
		if !fn(se.insID, se.endpoint) {
			return
		}
	}

	for _, se := range e.endpoints2 {
		if !fn(se.insID, se.endpoint) {
			return
		}
	}

	for _, se := range e.endpoints3 {
		if !fn(se.insID, se.endpoint) {
			return
		}
	}

	for _, se := range e.endpoints4 {
		if !fn(se.insID, se.endpoint) {
			return
		}
	}
}
