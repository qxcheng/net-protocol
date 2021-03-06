package fdbased

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/link/rawfile"
	"github.com/qxcheng/net-protocol/protocol/stack"
	"syscall"
)

// BufConfig 从NIC读取数据的多级缓存配置
var BufConfig = []int{128, 256, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768}

// 负责底层网卡的io读写以及数据分发
type endpoint struct {
	fd int                               // 发送和接收数据的文件描述符
	mtu uint32                           // 单个帧的最大长度
	hdrSize int                          // 以太网头部长度
	addr tcpip.LinkAddress               // 网卡mac地址
	caps stack.LinkEndpointCapabilities  // 网卡的能力

	// closed is a function to be called when the FD's peer (if any) closes
	// its end of the communication pipe.
	closed func(*tcpip.Error)

	iovecs     []syscall.Iovec
	views      []buffer.View
	dispatcher stack.NetworkDispatcher

	// handleLocal indicates whether packets destined to itself should be
	// handled by the netstack internally (true) or be forwarded to the FD
	// endpoint (false).
	// handleLocal指示发往自身的数据包是由内部netstack处理（true）还是转发到FD端点（false）。
	// Resend packets back to netstack if destined to itself
	// Add option to redirect packet back to netstack if it's destined to itself.
	// This fixes the problem where connecting to the local NIC address would
	// not work, e.g.:
	// echo bar | nc -l -p 8080 &
	// echo foo | nc 192.168.0.2 8080
	handleLocal bool
}

// Options 创建fdbase端的一些选项参数
type Options struct {
	FD                 int
	MTU                uint32
	ClosedFunc         func(*tcpip.Error)
	Address            tcpip.LinkAddress
	ResolutionRequired bool
	SaveRestore        bool
	ChecksumOffload    bool
	DisconnectOk       bool
	HandleLocal        bool
	TestLossPacket     func(data []byte) bool
}

func New(opts *Options) tcpip.LinkEndpointID {
	syscall.SetNonblock(opts.FD, true)  // TODO 为什么要设置非阻塞？

	// 根据参数设置支持的方法
	caps := stack.LinkEndpointCapabilities(0)
	if opts.ResolutionRequired {
		caps |= stack.CapabilityResolutionRequired
	}
	if opts.ChecksumOffload {
		caps |= stack.CapabilityChecksumOffload
	}
	if opts.SaveRestore {
		caps |= stack.CapabilitySaveRestore
	}
	if opts.DisconnectOk {
		caps |= stack.CapabilityDisconnectOk
	}

	e := &endpoint{
		fd:          opts.FD,
		mtu:         opts.MTU,
		hdrSize:     header.EthernetMinimumSize,
		addr:        opts.Address,
		caps:        caps,
		closed:      opts.ClosedFunc,
		iovecs:      make([]syscall.Iovec, len(BufConfig)),
		views:       make([]buffer.View, len(BufConfig)),
		handleLocal: opts.HandleLocal,
	}
	// 全局注册链路层设备, 返回链路层端点ID
	return stack.RegisterLinkEndpoint(e)
}

// Attach 启动从文件描述符中读取数据包的goroutine，并通过提供的分发函数来分发数据报。
// 实际传递的参数是：*NIC
func (e *endpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.dispatcher = dispatcher
	// 链接端点不可靠。保存传输端点后，它们将停止发送传出数据包，并拒绝所有传入数据包。
	go e.dispatchLoop()
}

func (e *endpoint) IsAttached() bool {
	return e.dispatcher != nil
}

func (e *endpoint) MTU() uint32 {
	return e.mtu
}

func (e *endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return e.caps
}

// MaxHeaderLength 返回当前以太网头部信息长度
func (e *endpoint) MaxHeaderLength() uint16 {
	return uint16(e.hdrSize)
}

// LinkAddress 返回当前MAC地址
func (e *endpoint) LinkAddress() tcpip.LinkAddress {
	return e.addr
}

// WritePacket 将上层的报文经过链路层封装，写入网卡中，如果写入失败则丢弃该报文
func (e *endpoint) WritePacket(r *stack.Route, hdr buffer.Prependable, payload buffer.VectorisedView,
	protocol tcpip.NetworkProtocolNumber) *tcpip.Error  {
	// 如果目标地址就是设备本身自己，那么将报文重新返回给协议栈
	if e.handleLocal && r.LocalAddress != "" && r.LocalAddress == r.RemoteAddress {
		views := make([]buffer.View, 1, 1+len(payload.Views()))
		views[0] = hdr.View()
		views = append(views, payload.Views()...)
		vv := buffer.NewVectorisedView(len(views[0])+payload.Size(), views)
		e.dispatcher.DeliverNetworkPacket(e, r.RemoteLinkAddress, r.LocalLinkAddress, protocol, vv)
		return nil
	}

	// 封装增加以太网头部
	eth := header.Ethernet(hdr.Prepend(header.EthernetMinimumSize))
	ethHdr := &header.EthernetFields{
		DstAddr: r.RemoteLinkAddress,
		Type: protocol,
	}

	// 如果路由信息中有配置源MAC地址，那么使用该地址，
	// 如果没有，则使用本网卡的地址
	if r.LocalLinkAddress != "" {
		ethHdr.SrcAddr = r.LocalLinkAddress
	} else {
		ethHdr.SrcAddr = e.addr
	}
	eth.Encode(ethHdr)

	// 写入网卡中
	// log.Printf("write to nic %d bytes", hdr.UsedLength()+payload.Size())
	if payload.Size() == 0 {
		return rawfile.NonBlockingWrite(e.fd, hdr.View())
	}

	return rawfile.NonBlockingWrite2(e.fd, hdr.View(), payload.ToView())
}

// 循环的从fd读取数据，然后将数据包分发给协议栈
func (e *endpoint) dispatchLoop() *tcpip.Error {
	for {
		cont, err := e.dispatch()
		if err != nil || !cont {
			if e.closed != nil {
				e.closed(err)
			}
			return err
		}
	}
}

// 从网卡中读取一个数据报
func (e *endpoint) dispatch() (bool, *tcpip.Error) {
	e.allocateViews(BufConfig)  // 读取数据缓存的分配
	// 从网卡中读取数据
	n, err := rawfile.BlockingReadv(e.fd, e.iovecs)
	if err != nil {
		return false, err
	}
	// 如果比头部长度还小，直接丢弃
	if n <= e.hdrSize {
		return false, nil
	}

	var (
		p tcpip.NetworkProtocolNumber
		remoteLinkAddr, localLinkAddr tcpip.LinkAddress
	)
	// 获取以太网头部信息
	eth := header.Ethernet(e.views[0])
	p = eth.Type()
	remoteLinkAddr = eth.SourceAddress()
	localLinkAddr = eth.DestinationAddress()

	used := e.capViews(n, BufConfig)
	vv := buffer.NewVectorisedView(n, e.views[:used])  // 完整的包
	// 将数据内容删除以太网头部信息，也就是将数据指针指向网络层的第一个字节
	vv.TrimFront(e.hdrSize)

	// 调用nic.DeliverNetworkPacket来分发网络层数据
	// log.Printf("read from nic %d bytes", e.hdrSize+vv.Size())
	e.dispatcher.DeliverNetworkPacket(e, remoteLinkAddr, localLinkAddr, p, vv)

	// 释放views, 为下一个包做准备
	for i := 0; i < used; i++ {
		e.views[i] = nil
	}

	return true, nil
}

// 按bufConfig的长度分配内存大小
// 注意e.views和e.iovecs共用相同的内存块
func (e *endpoint) allocateViews(bufConfig []int) {
	for i, v := range e.views {
		if v != nil {
			break
		}
		b := buffer.NewView(bufConfig[i])
		e.views[i] = b
		e.iovecs[i] = syscall.Iovec{
			Base: &b[0],
			Len:  uint64(len(b)),
		}
	}
}

// n:从网卡读取的字节数 buffers:就是BufConfig
func (e *endpoint) capViews(n int, buffers []int) int {
	c := 0
	for i, s := range buffers {
		c += s
		if c >= n {
			e.views[i].CapLength(s - (c - n))  // 将第i个缓存切片移除未使用的部分
			return i + 1                       // 返回使用的切片数
		}
	}
	return len(buffers)
}




