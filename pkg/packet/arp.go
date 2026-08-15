// Package packet provides allocation-free parsing and construction of Ethernet/ARP frames.
package packet

import "encoding/binary"

const (
	EthernetHeaderLen = 14
	ARPHeaderLen      = 28
	ARPFrameLen       = EthernetHeaderLen + ARPHeaderLen
	EtherTypeARP      = 0x0806
	ARPRequest        = 1
	ARPReply          = 2
)

// Request is the relevant data from an Ethernet/IPv4 ARP request.
type Request struct {
	SenderMAC [6]byte
	SenderIP  [4]byte
	TargetIP  [4]byte
}

// ParseRequest accepts only Ethernet II ARP requests for Ethernet and IPv4.
// It rejects malformed, reply, unknown, and gratuitous ARP packets.
func ParseRequest(frame []byte) (Request, bool) {
	var r Request
	if len(frame) < ARPFrameLen || binary.BigEndian.Uint16(frame[12:14]) != EtherTypeARP {
		return r, false
	}
	a := frame[EthernetHeaderLen:]
	if binary.BigEndian.Uint16(a[0:2]) != 1 || // Ethernet
		binary.BigEndian.Uint16(a[2:4]) != 0x0800 || // IPv4
		a[4] != 6 || a[5] != 4 || binary.BigEndian.Uint16(a[6:8]) != ARPRequest {
		return r, false
	}
	copy(r.SenderMAC[:], a[8:14])
	copy(r.SenderIP[:], a[14:18])
	copy(r.TargetIP[:], a[24:28])
	if r.SenderIP == r.TargetIP { // gratuitous ARP request/probe announcement
		return Request{}, false
	}
	return r, true
}

// BuildReply writes a complete 42-byte Ethernet/ARP reply into dst.
func BuildReply(dst []byte, req Request, replyMAC [6]byte) bool {
	if len(dst) < ARPFrameLen {
		return false
	}
	copy(dst[0:6], req.SenderMAC[:])
	copy(dst[6:12], replyMAC[:])
	binary.BigEndian.PutUint16(dst[12:14], EtherTypeARP)
	a := dst[EthernetHeaderLen:ARPFrameLen]
	binary.BigEndian.PutUint16(a[0:2], 1)
	binary.BigEndian.PutUint16(a[2:4], 0x0800)
	a[4], a[5] = 6, 4
	binary.BigEndian.PutUint16(a[6:8], ARPReply)
	copy(a[8:14], replyMAC[:])
	copy(a[14:18], req.TargetIP[:])
	copy(a[18:24], req.SenderMAC[:])
	copy(a[24:28], req.SenderIP[:])
	return true
}
