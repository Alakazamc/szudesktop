package vpn

import (
	"context"
	"net"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const (
	defaultNIC tcpip.NICID = 1
	defaultMTU uint32      = 1400
)

// Endpoint 是 gVisor 的链路层端点：把隧道收来的裸 IPv4 包喂给栈，
// 把栈发出的包回调给发流。
type Endpoint struct {
	dispatcher stack.NetworkDispatcher
	onRecv     func([]byte)
}

func (ep *Endpoint) MTU() uint32                    { return defaultMTU }
func (ep *Endpoint) MaxHeaderLength() uint16        { return 0 }
func (ep *Endpoint) LinkAddress() tcpip.LinkAddress { return "" }
func (ep *Endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}
func (ep *Endpoint) Attach(d stack.NetworkDispatcher)        { ep.dispatcher = d }
func (ep *Endpoint) IsAttached() bool                        { return ep.dispatcher != nil }
func (ep *Endpoint) Wait()                                   {}
func (ep *Endpoint) ARPHardwareType() header.ARPHardwareType { return header.ARPHardwareNone }
func (ep *Endpoint) AddHeader(*stack.PacketBuffer)           {}
func (ep *Endpoint) Close()                                  {}
func (ep *Endpoint) SetOnCloseAction(func())                 {}
func (ep *Endpoint) ParseHeader(*stack.PacketBuffer) bool    { return true }
func (ep *Endpoint) SetMTU(uint32)                           {}
func (ep *Endpoint) SetLinkAddress(tcpip.LinkAddress)        {}

// WritePackets：栈发出来的包（去往校内网），交给发流回调。
func (ep *Endpoint) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	n := 0
	for _, pb := range list.AsSlice() {
		buf := make([]byte, 0, pb.Size())
		for _, s := range pb.AsSlices() {
			buf = append(buf, s...)
		}
		if len(buf) > 0 && ep.onRecv != nil {
			ep.onRecv(buf)
		}
		n++
	}
	return n, nil
}

// WriteTo：隧道收来的包（来自校内网），塞给 gVisor 栈。
func (ep *Endpoint) WriteTo(buf []byte) {
	if !ep.IsAttached() {
		return
	}
	pb := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithData(buf),
	})
	ep.dispatcher.DeliverNetworkPacket(header.IPv4ProtocolNumber, pb)
	pb.DecRef()
}

// SetOnRecv 注册发流回调（nil = 摘除）。
func (ep *Endpoint) SetOnRecv(fn func([]byte)) { ep.onRecv = fn }

// setupStack 建用户态 IP 栈：本机 IP/32 + 默认路由全走这条 NIC。
func setupStack(ip []byte, endpoint *Endpoint) *stack.Stack {
	ipStack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
		HandleLocal:        true,
	})
	if err := ipStack.CreateNIC(defaultNIC, endpoint); err != nil {
		panic("vpn: CreateNIC: " + err.String())
	}
	addr := tcpip.AddrFromSlice(ip)
	protoAddr := tcpip.ProtocolAddress{
		AddressWithPrefix: tcpip.AddressWithPrefix{Address: addr, PrefixLen: 32},
		Protocol:          ipv4.ProtocolNumber,
	}
	if err := ipStack.AddProtocolAddress(defaultNIC, protoAddr, stack.AddressProperties{}); err != nil {
		panic("vpn: AddProtocolAddress: " + err.String())
	}
	sOpt := tcpip.TCPSACKEnabled(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sOpt)
	cOpt := tcpip.CongestionControlOption("cubic")
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &cOpt)
	ipStack.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: defaultNIC})
	return ipStack
}

// dialViaStack 在用户态栈里向 内网IP:端口 发起 TCP 连接（SOCKS5 的 Dialer 用）。
func dialViaStack(ipStack *stack.Stack, selfIp []byte, targetIP net.IP, port uint16) (net.Conn, error) {
	addrTarget := tcpip.FullAddress{
		NIC:  defaultNIC,
		Port: port,
		Addr: tcpip.AddrFromSlice(targetIP),
	}
	bind := tcpip.FullAddress{
		NIC:  defaultNIC,
		Addr: tcpip.AddrFromSlice(selfIp),
	}
	return gonet.DialTCPWithBind(context.Background(), ipStack, bind, addrTarget, header.IPv4ProtocolNumber)
}
