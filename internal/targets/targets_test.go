package targets

import (
	"fmt"
	"strings"
	"testing"
)
func TestParseContainsAndMerge(t *testing.T){
	s,err:=Parse(strings.NewReader("# comment\n192.0.2.1\n192.0.2.0/24\n\n10.0.0.1\n"));if err!=nil{t.Fatal(err)}
	for _,a:=range [][4]byte{{192,0,2,200},{10,0,0,1}}{if !s.Contains(a){t.Fatalf("missing %v",a)}}
	if s.Contains([4]byte{10,0,0,2}){t.Fatal("unexpected match")}
	if s.LenRanges()!=2{t.Fatalf("ranges=%d",s.LenRanges())}
}
func TestParseRejectsIPv6AndBadLines(t *testing.T){for _,v:=range []string{"2001:db8::1\n","not-an-ip\n","10.0.0.0/99\n"}{if _,err:=Parse(strings.NewReader(v));err==nil{t.Fatalf("accepted %q",v)}}}
func BenchmarkContainsThousands(b *testing.B){
	var in strings.Builder; for i:=0;i<4096;i++ { fmt.Fprintf(&in, "10.%d.%d.1\n", (i/256)%256, i%256) }
	s,_:=Parse(strings.NewReader(in.String()));a:=[4]byte{192,0,2,1};b.ReportAllocs();for i:=0;i<b.N;i++{s.Contains(a)}
}
