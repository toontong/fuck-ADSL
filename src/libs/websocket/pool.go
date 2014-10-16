package websocket

import (
	"errors"
	"sync/atomic"
)

type Pool struct {
	conns  chan *Conn
	readCh chan *msgitem

	count int32
	//lock  sync.RWMutex
}

func NewPool() *Pool {
	p := new(Pool)
	p.conns = make(chan *Conn, 1024)
	p.readCh = make(chan *msgitem, 1024)
	return p
}

func (p *Pool) ReadMessage() (messageType byte, message []byte, err error) {

	item, ok := <-p.readCh

	if !ok {
		return 0, nil, ErrDisconnected
	}

	return item.messageType, item.message, item.err
}

func (p *Pool) WriteMessage(messageType byte, message []byte) error {
	for {
		if p.count <= 0 {
			return ErrDisconnected
		}

		c, _ := <-p.conns

		err := c.WriteMessage(messageType, message)

		if err == nil {
			p.conns <- c
			break
		}
	}
	return nil
}

func (p *Pool) Run(c *Conn) {
	atomic.AddInt32(&p.count, 1)
	p.conns <- c

	for {
		messageType, message, err := c.ReadMessage()

		p.readCh <- &msgitem{messageType: messageType, message: message, err: err}

		if err != nil {
			break
		}

		if messageType == CloseMessage {
			break
		}
	}
	atomic.AddInt32(&p.count, -1)
	c.Close()
}

func (p *Pool) Count() int {
	return int(p.count)
}

type msgitem struct {
	messageType byte
	message     []byte
	err         error
}

var (
	ErrDisconnected = errors.New("All connections were disconnected")
	ErrTooBusy      = errors.New("Connections too busy")
)
