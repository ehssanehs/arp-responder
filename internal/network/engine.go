package network

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/ehssanehs/arp-responder/internal/metrics"
	"github.com/ehssanehs/arp-responder/internal/targets"
	"github.com/ehssanehs/arp-responder/pkg/packet"
	"golang.org/x/sys/unix"
)

const ethernetProtocolARP = 0x0806

type RuntimeConfig struct {
	Interfaces []string
	ReplyMAC   string
	Targets    *targets.Set
	result     chan error
}
type socket struct { fd, ifindex int; mac [6]byte; name string }
type Engine struct {
	epfd, wakefd int
	sockets map[int]*socket
	pending atomic.Pointer[RuntimeConfig]
	stopping atomic.Bool
	log *slog.Logger
	metrics *metrics.Metrics
	active *activeConfig
}

func New(log *slog.Logger, m *metrics.Metrics) (*Engine, error) {
	epfd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil { return nil, fmt.Errorf("epoll_create: %w", err) }
	wakefd, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil { unix.Close(epfd); return nil, fmt.Errorf("eventfd: %w", err) }
	if err = unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, wakefd, &unix.EpollEvent{Events:unix.EPOLLIN, Fd:int32(wakefd)}); err != nil {
		unix.Close(wakefd); unix.Close(epfd); return nil, err
	}
	return &Engine{epfd:epfd,wakefd:wakefd,sockets:make(map[int]*socket),log:log,metrics:m}, nil
}
// Initialize installs the first configuration before the event loop starts.
func (e *Engine) Initialize(c *RuntimeConfig) error { return e.apply(c) }

func (e *Engine) Update(c *RuntimeConfig) error {
	if c.Targets == nil { return errors.New("nil target set") }
	copyConfig := &RuntimeConfig{Interfaces: append([]string(nil), c.Interfaces...), ReplyMAC: c.ReplyMAC, Targets: c.Targets, result: make(chan error, 1)}
	e.pending.Store(copyConfig)
	if err := e.wake(); err != nil { return err }
	return <-copyConfig.result
}
func (e *Engine) Stop() { e.stopping.Store(true); _ = e.wake() }
func (e *Engine) wake() error {
	var b [8]byte; b[0]=1
	_, err := unix.Write(e.wakefd,b[:]); if err == unix.EAGAIN { return nil }; return err
}

func (e *Engine) Run() error {
	defer e.closeAll()
	var events [64]unix.EpollEvent
	var receive [2048]byte
	var reply [packet.ARPFrameLen]byte
	for {
		n, err := unix.EpollWait(e.epfd,events[:],-1)
		if err == unix.EINTR { continue }
		if err != nil { return fmt.Errorf("epoll_wait: %w",err) }
		for i:=0;i<n;i++ {
			fd:=int(events[i].Fd)
			if fd==e.wakefd {
				e.drainWake()
				if e.stopping.Load() { return nil }
				if c := e.pending.Swap(nil); c != nil {
					applyErr := e.apply(c)
					c.result <- applyErr
					if applyErr != nil { e.log.Error("network configuration rejected", "error", applyErr) }
				}
				continue
			}
			s:=e.sockets[fd]; if s==nil { continue }
			e.drain(s,receive[:],reply[:])
		}
	}
}
func (e *Engine) drain(s *socket, receive, reply []byte) {
	for {
		n, packetType, err := recvPacket(s.fd, receive)
		if err==unix.EAGAIN || err==unix.EWOULDBLOCK { return }
		if err!=nil { e.metrics.Dropped.Inc(); e.log.Error("packet receive failed","interface",s.name,"error",err); return }
		if packetType == unix.PACKET_OUTGOING { continue }
		req,ok:=packet.ParseRequest(receive[:n]); if !ok { e.metrics.Invalid.Inc(); continue }
		e.metrics.Requests.Inc()
		active := e.active
		if active==nil || !active.Targets.Contains(req.TargetIP) { continue }
		mac:=s.mac
		if active.ReplyMAC!="auto" { mac=active.fixedMAC }
		packet.BuildReply(reply,req,mac)
		if err := sendPacket(s.fd, s.ifindex, req.SenderMAC, reply); err != nil { e.metrics.Dropped.Inc(); continue }
		e.metrics.Replies.Inc()
	}
}

type activeConfig struct { Targets *targets.Set; ReplyMAC string; fixedMAC [6]byte }


func (e *Engine) apply(c *RuntimeConfig) error {
	wanted := make(map[string]struct{}, len(c.Interfaces))
	for _, name := range c.Interfaces { wanted[name]=struct{}{} }
	byName := make(map[string]*socket,len(e.sockets))
	for _, s := range e.sockets { byName[s.name]=s }
	var opened []*socket
	for _, name := range c.Interfaces {
		if byName[name]!=nil { continue }
		s,err:=openSocket(name)
		if err!=nil { for _,x:=range opened { unix.Close(x.fd) }; return err }
		opened=append(opened,s)
	}
	// Register all new descriptors before changing the live set.
	for i,s:=range opened {
		if err:=unix.EpollCtl(e.epfd,unix.EPOLL_CTL_ADD,s.fd,&unix.EpollEvent{Events:unix.EPOLLIN,Fd:int32(s.fd)}); err!=nil {
			for _,x:=range opened[i:] { unix.Close(x.fd) }
			for _,x:=range opened[:i] { unix.EpollCtl(e.epfd,unix.EPOLL_CTL_DEL,x.fd,nil); unix.Close(x.fd) }
			return fmt.Errorf("epoll add %s: %w",s.name,err)
		}
		e.sockets[s.fd]=s
	}
	for fd,s:=range e.sockets {
		if _,ok:=wanted[s.name]; !ok { unix.EpollCtl(e.epfd,unix.EPOLL_CTL_DEL,fd,nil); unix.Close(fd); delete(e.sockets,fd); e.log.Info("interface detached","interface",s.name) }
	}
	for _,s:=range opened { e.log.Info("interface attached","interface",s.name,"mac",net.HardwareAddr(s.mac[:]).String()) }
	a:=&activeConfig{Targets:c.Targets,ReplyMAC:c.ReplyMAC}
	if c.ReplyMAC!="auto" { m,_:=net.ParseMAC(c.ReplyMAC); copy(a.fixedMAC[:],m) }
	e.active = a
	return nil
}

func openSocket(name string) (*socket,error) {
	iface,err:=net.InterfaceByName(name); if err!=nil { return nil,fmt.Errorf("interface %s: %w",name,err) }
	if len(iface.HardwareAddr)!=6 { return nil,fmt.Errorf("interface %s does not have a 6-byte Ethernet MAC",name) }
	fd,err:=unix.Socket(unix.AF_PACKET,unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC,int(htons(ethernetProtocolARP)))
	if err!=nil { return nil,fmt.Errorf("socket %s: %w",name,err) }
	ok:=false
	defer func(){ if !ok { unix.Close(fd) } }()
	_ = unix.SetsockoptInt(fd,unix.SOL_SOCKET,unix.SO_RCVBUF,4*1024*1024)
	_ = unix.SetsockoptInt(fd,unix.SOL_PACKET,unix.PACKET_IGNORE_OUTGOING,1)
	_ = unix.SetsockoptInt(fd,unix.SOL_PACKET,unix.PACKET_QDISC_BYPASS,1)
	if err=unix.Bind(fd,&unix.SockaddrLinklayer{Protocol:htons(ethernetProtocolARP),Ifindex:iface.Index}); err!=nil { return nil,fmt.Errorf("bind %s: %w",name,err) }
	s:=&socket{fd:fd,ifindex:iface.Index,name:name}; copy(s.mac[:],iface.HardwareAddr); ok=true; return s,nil
}
func (e *Engine) drainWake(){ var b [8]byte; for { _,err:=unix.Read(e.wakefd,b[:]); if err!=nil{return} } }
func (e *Engine) closeAll(){ for fd:=range e.sockets { unix.Close(fd) }; unix.Close(e.wakefd); unix.Close(e.epfd) }
func htons(v uint16) uint16 { return v<<8 | v>>8 }


func recvPacket(fd int, buf []byte) (int, uint8, error) {
	var address unix.RawSockaddrLinklayer
	addressLength := uint32(unix.SizeofSockaddrLinklayer)
	r0, _, errno := unix.Syscall6(unix.SYS_RECVFROM, uintptr(fd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unix.MSG_DONTWAIT, uintptr(unsafe.Pointer(&address)), uintptr(unsafe.Pointer(&addressLength)))
	runtime.KeepAlive(buf)
	if errno != 0 { return 0, 0, errno }
	return int(r0), address.Pkttype, nil
}

func sendPacket(fd, ifindex int, destination [6]byte, frame []byte) error {
	address := unix.RawSockaddrLinklayer{Family: unix.AF_PACKET, Protocol: htons(ethernetProtocolARP), Ifindex: int32(ifindex), Halen: 6}
	copy(address.Addr[:], destination[:])
	_, _, errno := unix.Syscall6(unix.SYS_SENDTO, uintptr(fd), uintptr(unsafe.Pointer(&frame[0])), uintptr(len(frame)), 0, uintptr(unsafe.Pointer(&address)), unix.SizeofSockaddrLinklayer)
	runtime.KeepAlive(frame)
	if errno != 0 { return errno }
	return nil
}
