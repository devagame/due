package gate

import (
	"context"

	"github.com/devagame/due/v2/cluster"
	"github.com/devagame/due/v2/core/buffer"
	"github.com/devagame/due/v2/errors"
	"github.com/devagame/due/v2/internal/link"
	"github.com/devagame/due/v2/log"
	"github.com/devagame/due/v2/mode"
	"github.com/devagame/due/v2/packet"
)

type proxy struct {
	gate       *Gate            // 网关服
	nodeLinker *link.NodeLinker // 节点链接器
}

func newProxy(gate *Gate) *proxy {
	return &proxy{gate: gate, nodeLinker: link.NewNodeLinker(gate.ctx, &link.Options{
		InsID:    gate.opts.id,
		InsKind:  cluster.Gate,
		Locator:  gate.opts.locator,
		Registry: gate.opts.registry,
		Dispatch: gate.opts.dispatch,
	})}
}

// 绑定用户与网关间的关系
func (p *proxy) bindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.BindGate(ctx, uid, p.gate.opts.id)
	if err != nil {
		return err
	}

	p.trigger(ctx, cluster.Reconnect, cid, uid)

	return nil
}

// 解绑用户与网关间的关系
func (p *proxy) unbindGate(ctx context.Context, cid, uid int64) error {
	err := p.gate.opts.locator.UnbindGate(ctx, uid, p.gate.opts.id)
	if err != nil {
		log.Errorf("user unbind failed, gid: %s, cid: %d, uid: %d, err: %v", p.gate.opts.id, cid, uid, err)
	}

	return err
}

// 触发事件
func (p *proxy) trigger(ctx context.Context, event cluster.Event, cid, uid int64) {
	if mode.IsDebugMode() {
		log.Debugf("trigger event, event: %v cid: %d uid: %d", event.String(), cid, uid)
	}

	if err := p.nodeLinker.Trigger(ctx, &link.TriggerArgs{
		Event: event,
		CID:   cid,
		UID:   uid,
	}); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundEvent), errors.Is(err, errors.ErrNotFoundUserLocation):
			log.Warnf("trigger event failed, cid: %d, uid: %d, event: %v, err: %v", cid, uid, event.String(), err)
		default:
			log.Errorf("trigger event failed, cid: %d, uid: %d, event: %v, err: %v", cid, uid, event.String(), err)
		}
	}
}

// 投递消息
func (p *proxy) deliver(ctx context.Context, cid, uid int64, buf buffer.Buffer) {
	message, err := packet.UnpackMessage(buf.Bytes())
	if err != nil {
		log.Errorf("unpack message failed: %v", err)
		return
	}

	if mode.IsDebugMode() {
		log.Debugf("deliver message, cid: %d uid: %d seq: %d route: %d buffer: %s", cid, uid, message.Seq, message.Route, string(message.Buffer))
	}

	if err = p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
		CID:    cid,
		UID:    uid,
		Route:  message.Route,
		Buffer: buf,
	}); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundRoute), errors.Is(err, errors.ErrNotFoundEndpoint):
			log.Warnf("deliver message failed, cid: %d uid: %d seq: %d route: %d err: %v", cid, uid, message.Seq, message.Route, err)
		default:
			log.Errorf("deliver message failed, cid: %d uid: %d seq: %d route: %d err: %v", cid, uid, message.Seq, message.Route, err)
		}
	}
}

// 开始监听
func (p *proxy) watch() {
	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
