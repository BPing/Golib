package producerConsumer

import (
	"errors"
	"time"
)

// 容器
// 实现基于生产/消费模式
//
//   1、Produce(msg interface{}) 生产信息，把消息放入消息列表中。
//   2、Consume() 消费消息。
//
// 开启主线程一直消费消息，如果消息过多时（消息队列满），则会开启协助协程消费消息。
// 协助协程将会在消息队列持续为空一段的时间后关闭.
type Container struct {
	// 消费信息的函数
	// 信息体最终落到此函数处理
	// 由用户自定义函数实体内容
	consumeFunc func(Message)

	// 消息体队列存放的channel
	// 长度由用户自定义
	// 建议长度大于等于1，
	// 使用有缓冲的channel
	msgList chan Message

	// 空闲存活时间（针对AssistRunner）
	// 当协助消费协程空闲时间超过此
	// 限定时间，则被关闭。默认为1s
	assistIdleKeepAlive time.Duration
}

// 消息结构体
type Message struct {
	// 标识
	// 此字段不能为空，
	// 否则会被当做无效数据抛弃。
	Id string
	// 实体内容
	// 用户自定义的消息具体信息内容
	Body interface{}
}

const (
	MasterRunner = true
	AssistRunner = false
)

var (
	// 错误信息
	MsgListNilErr     = errors.New("list of message is nil ")
	ChanLenErr        = errors.New("length of chan should be bigger than one")
	ConsumeFuncNilErr = errors.New("func of consumer should not be nil")
	MessageIDNilErr   = errors.New("Id of Message should not be nil")
)

// 新建生产/消费模式容器
func NewContainerPC(chanLen int, consumeFunc func(Message)) (*Container, error) {
	if consumeFunc == nil {
		return nil, ConsumeFuncNilErr
	}
	if chanLen < 1 {
		return nil, ChanLenErr
	}

	return &Container{consumeFunc: consumeFunc, msgList: make(chan Message, chanLen), assistIdleKeepAlive: time.Second}, nil
}

// 新建消息体
func NewMessage(id string, body interface{}) (Message, error) {
	if id == "" {
		return Message{}, MessageIDNilErr
	}
	return Message{id, body}, nil
}

// 设置空闲存活时间
func (c *Container) SetAssistIdleKeepAlive(timeout time.Duration) {
	if timeout > 0 {
		c.assistIdleKeepAlive = timeout
	}
}

// 生产
// 如果队列已满，开启新协助协程消费消息
func (c *Container) Produce(msg Message) error {
	if nil == c.msgList {
		return MsgListNilErr
	}
	select {
	case c.msgList <- msg:
	default:
		c.consume(AssistRunner,&msg)
	}
	return nil
}

// 消费
// 一般调用一次即可
// 因为每一次调用都会开启一个主消费协程
func (c *Container) Consume() error {
	if nil == c.msgList {
		return MsgListNilErr
	}
	c.consume(MasterRunner,nil)
	return nil
}

// 开启消费协程
// @master 是否主要消费协程。
//         主要消费协程一直执行
//         协助协程是在消息过多的时候开启，在没有消息体的时候结束。
// @argMsg 队列已满，放不进去的消息，协助协程消费的第一个消息。
func (c *Container) consume(master bool,argMsg *Message) {
	if master == MasterRunner {
		// 主要消费协程
		go func() {
			var msg Message
			for {
				// 一直消费消息，如果队列没有消息，则阻塞等待
				msg = <-c.msgList
				if nil != c.consumeFunc && msg.Id != "" {
					//debug msg.Id = "主要" + msg.Id
					c.consumeFunc(msg)
				}
			}
		}()
	} else {
		if c.assistIdleKeepAlive <= 0 {
			// 默认一秒
			c.assistIdleKeepAlive = time.Second
		}
		// 协助消费协程
		go func() {
			//先消费放不进队列的消息
			if nil != c.consumeFunc &&nil!=argMsg&&argMsg.Id != "" {
				c.consumeFunc(*argMsg)
			}
			var msg Message
			for {
				select {
				case msg = <-c.msgList:
					if nil != c.consumeFunc && msg.Id != "" {
						//debug msg.Id = "协助" + msg.Id
						c.consumeFunc(msg)
					}
				case <-time.After(c.assistIdleKeepAlive):
					//如果队列没有消息，空闲时间超过c.assistIdleKeepAlive，此协程协助使命结束
					return
				}
			}
		}()
	}

}
