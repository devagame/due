package tcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devagame/due/v2/errors"
	"github.com/devagame/due/v2/log"
	"github.com/devagame/due/v2/network"
	"github.com/devagame/due/v2/packet"
	"github.com/devagame/due/v2/utils/xcall"
	"github.com/devagame/due/v2/utils/xnet"
	"github.com/devagame/due/v2/utils/xtime"
)

type serverConn struct {
	id                int64          // 连接ID
	uid               atomic.Int64   // 用户ID
	attr              *attr          // 连接属性
	state             atomic.Int32   // 连接状态
	connMgr           *serverConnMgr // 连接管理
	rw                sync.RWMutex   // 读写锁
	conn              net.Conn       // TCP源连接
	chWrite           chan chWrite   // 写入队列
	done              chan struct{}  // 写入完成信号
	close             chan struct{}  // 关闭信号
	lastHeartbeatTime atomic.Int64   // 上次心跳时间
	authorizeTimer    atomic.Value   // 授权定时器
}

var _ network.Conn = &serverConn{}

// ID 获取连接ID
func (c *serverConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
func (c *serverConn) Bind(uid int64) {
	c.uid.Store(uid)

	c.uncheckAuthorize()
}

// Unbind 解绑用户ID
func (c *serverConn) Unbind() {
	c.uid.Store(0)

	c.checkAuthorize()
}

// Send 发送消息（同步）
func (c *serverConn) Send(msg []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	_, err := conn.Write(msg)
	return err
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}

	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.conn == nil {
		return errors.ErrConnectionClosed
	}

	c.chWrite <- chWrite{typ: dataPacket, msg: msg}

	return nil
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose(true)
	} else {
		return c.graceClose(true)
	}
}

// LocalIP 获取本地IP
func (c *serverConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
func (c *serverConn) LocalAddr() (net.Addr, error) {
	if err := c.checkState(); err != nil {
		return nil, err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return nil, errors.ErrConnectionClosed
	}

	return conn.LocalAddr(), nil
}

// RemoteIP 获取远端IP
func (c *serverConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
func (c *serverConn) RemoteAddr() (net.Addr, error) {
	if err := c.checkState(); err != nil {
		return nil, err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return nil, errors.ErrConnectionClosed
	}

	return conn.RemoteAddr(), nil
}

// 检测连接状态
func (c *serverConn) checkState() error {
	switch c.State() {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// 授权检查
func (c *serverConn) checkAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
			if c.UID() != 0 {
				return
			}

			c.forceClose(true)
		}))
		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// 取消授权检查
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		timer := c.authorizeTimer.Swap((*time.Timer)(nil))

		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// 初始化连接
func (c *serverConn) init(cm *serverConnMgr, id int64, conn net.Conn) {
	c.id = id
	c.uid.Store(0)
	c.attr = &attr{}
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.connMgr = cm
	c.chWrite = make(chan chWrite, 4096)
	c.done = make(chan struct{})
	c.close = make(chan struct{})
	c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))

	xcall.Go(c.read)

	xcall.Go(c.write)

	c.checkAuthorize()

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
}

// 重置连接
func (c *serverConn) reset() {
	c.attr = nil
}

// 优雅关闭
func (c *serverConn) graceClose(isNeedRecycle bool) error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.uncheckAuthorize()

	c.rw.RLock()
	if c.conn == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	c.chWrite <- chWrite{typ: closeSig}
	c.rw.RUnlock()

	<-c.done

	if !c.state.CompareAndSwap(int32(network.ConnHanged), int32(network.ConnClosed)) {
		return errors.ErrConnectionNotHanged
	}

	return c.doClose(isNeedRecycle)
}

// 强制关闭
func (c *serverConn) forceClose(isNeedRecycle bool) error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnClosed)) {
		if !c.state.CompareAndSwap(int32(network.ConnHanged), int32(network.ConnClosed)) {
			return errors.ErrConnectionClosed
		}
	}

	c.uncheckAuthorize()

	return c.doClose(isNeedRecycle)
}

// 执行关闭操作
func (c *serverConn) doClose(isNeedRecycle bool) error {
	c.rw.Lock()

	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	close(c.chWrite)
	close(c.close)
	close(c.done)
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	err := conn.Close()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	if isNeedRecycle {
		c.connMgr.recycle(conn)
	}

	return err
}

// 读取消息
func (c *serverConn) read() {
	conn := c.conn

	for {
		select {
		case <-c.close:
			return
		default:
			buf, err := packet.ReadBuffer(conn)
			if err != nil {
				_ = c.forceClose(true)
				return
			}

			if c.connMgr.server.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
			}

			switch c.State() {
			case network.ConnHanged:
				continue
			case network.ConnClosed:
				return
			default:
				// ignore
			}

			// ignore empty packet
			if buf.Len() == 0 {
				continue
			}

			isHeartbeat, err := packet.CheckHeartbeat(buf.Bytes())
			if err != nil {
				log.Errorf("check heartbeat message error: %v", err)
				continue
			}

			// ignore heartbeat packet
			if isHeartbeat {
				// responsive heartbeat
				if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
					c.sendHeartbeat(conn)
				}
			} else {
				if c.connMgr.server.receiveHandler != nil {
					c.connMgr.server.receiveHandler(c, buf)
				}
			}
		}
	}
}

// 写入消息
func (c *serverConn) write() {
	var (
		conn   = c.conn
		ticker *time.Ticker
	)

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		ticker = time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{C: make(chan time.Time, 1)}
	}

	for {
		select {
		case r, ok := <-c.chWrite:
			if !ok {
				return
			}

			if r.typ == closeSig {
				c.rw.RLock()
				if c.conn != nil {
					c.done <- struct{}{}
				}
				c.rw.RUnlock()
				return
			}

			if c.isClosed() {
				return
			}

			if _, err := conn.Write(r.msg); err != nil {
				log.Errorf("write data message error: %v", err)
			}
		case t, ok := <-ticker.C:
			if !ok {
				return
			}

			deadline := t.Add(-2 * c.connMgr.server.opts.heartbeatInterval).UnixNano()

			if c.lastHeartbeatTime.Load() < deadline {
				log.Debugf("connection heartbeat timeout, cid: %d", c.id)
				_ = c.forceClose(true)
				return
			} else {
				if c.isClosed() {
					return
				}

				if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
					c.sendHeartbeat(conn)
				}
			}
		}
	}
}

// 是否已关闭
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}

// 发送心跳包
func (c *serverConn) sendHeartbeat(conn net.Conn) {
	if heartbeat, err := packet.PackHeartbeat(); err != nil {
		log.Errorf("pack heartbeat message error: %v", err)
	} else {
		if _, err = conn.Write(heartbeat); err != nil {
			log.Errorf("write heartbeat message error: %v", err)
		}
	}
}
