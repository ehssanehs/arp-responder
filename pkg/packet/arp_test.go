package packet

import (
	"encoding/binary"
	"testing"
)

func requestFrame() [ARPFrameLen]byte {
	var f [ARPFrameLen]byte
	copy(f[0:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	copy(f[6:12], []byte{0x02, 0, 0, 0, 0, 1})
	binary.BigEndian.PutUint16(f[12:14], EtherTypeARP)
	a := f[14:]
	binary.BigEndian.PutUint16(a[0:2], 1)
	binary.BigEndian.PutUint16(a[2:4], 0x0800)
	a[4], a[5] = 6, 4
	binary.BigEndian.PutUint16(a[6:8], ARPRequest)
	copy(a[8:14], f[6:12])
	copy(a[14:18], []byte{192, 0, 2, 10})
	copy(a[24:28], []byte{192, 0, 2, 20})
	return f
}

func TestParseAndBuildReply(t *testing.T) {
	f := requestFrame()
	r, ok := ParseRequest(f[:])
	if !ok || r.TargetIP != [4]byte{192, 0, 2, 20} { t.Fatalf("unexpected parse: %+v %v", r, ok) }
	var out [ARPFrameLen]byte
	mac := [6]byte{0x02, 1, 2, 3, 4, 5}
	if !BuildReply(out[:], r, mac) { t.Fatal("build failed") }
	if binary.BigEndian.Uint16(out[20:22]) != ARPReply { t.Fatal("not an ARP reply") }
	if got := [4]byte(out[28:32]); got != r.TargetIP { t.Fatalf("sender IP = %v", got) }
	if got := [6]byte(out[0:6]); got != r.SenderMAC { t.Fatalf("destination MAC = %v", got) }
}
func TestParseRejectsMalformedAndIgnored(t *testing.T) {
	f := requestFrame()
	cases := [][]byte{nil, f[:20]}
	for _, c := range cases { if _, ok := ParseRequest(c); ok { t.Fatal("accepted malformed frame") } }
	binary.BigEndian.PutUint16(f[20:22], ARPReply)
	if _, ok := ParseRequest(f[:]); ok { t.Fatal("accepted reply") }
	f = requestFrame(); copy(f[38:42], f[28:32])
	if _, ok := ParseRequest(f[:]); ok { t.Fatal("accepted gratuitous request") }
}
func BenchmarkProcess100000(b *testing.B) {
	f := requestFrame(); var out [ARPFrameLen]byte; mac := [6]byte{2,1,2,3,4,5}
	b.ReportAllocs()
	for n:=0;n<b.N;n++ { for i:=0;i<100000;i++ { r,ok:=ParseRequest(f[:]);if ok{BuildReply(out[:],r,mac)} } }
}
func TestStress100000(t *testing.T) {
	f:=requestFrame();var out [ARPFrameLen]byte
	for i:=0;i<100000;i++ { r,ok:=ParseRequest(f[:]);if !ok||!BuildReply(out[:],r,[6]byte{2,1,2,3,4,5}){t.Fatal("processing failed")} }
}
