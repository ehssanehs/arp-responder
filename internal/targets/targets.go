package targets

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strings"
)

type interval struct{ lo, hi uint32 }

// Set is an immutable, compact set of merged IPv4 address intervals.
type Set struct{ ranges []interval }

func Parse(r io.Reader) (*Set, error) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1024*1024)
	var ranges []interval
	for line := 1; s.Scan(); line++ {
		text := strings.TrimSpace(strings.SplitN(s.Text(), "#", 2)[0])
		if text == "" {
			continue
		}
		p, err := parseTarget(text)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		a := p.Addr().As4()
		lo := toUint32(a)
		host := 32 - p.Bits()
		var hi uint32
		if host == 32 {
			hi = ^uint32(0)
		} else {
			hi = lo | (uint32(1)<<host - 1)
		}
		ranges = append(ranges, interval{lo: lo, hi: hi})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })
	merged := ranges[:0]
	for _, cur := range ranges {
		if len(merged) == 0 || (merged[len(merged)-1].hi != ^uint32(0) && cur.lo > merged[len(merged)-1].hi+1) {
			merged = append(merged, cur)
		} else if cur.hi > merged[len(merged)-1].hi {
			merged[len(merged)-1].hi = cur.hi
		}
	}
	return &Set{ranges: merged}, nil
}

func parseTarget(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil || !p.Addr().Is4() {
			return netip.Prefix{}, fmt.Errorf("invalid IPv4 prefix %q", s)
		}
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil || !a.Is4() {
		return netip.Prefix{}, fmt.Errorf("invalid IPv4 address %q", s)
	}
	return netip.PrefixFrom(a, 32), nil
}

func (s *Set) Contains(a [4]byte) bool {
	x := toUint32(a)
	i := sort.Search(len(s.ranges), func(i int) bool { return s.ranges[i].lo > x })
	return i > 0 && x <= s.ranges[i-1].hi
}
func (s *Set) LenRanges() int { return len(s.ranges) }
func toUint32(a [4]byte) uint32 { return uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3]) }

// ParseBytes parses target file contents.
func ParseBytes(b []byte) (*Set,error) { return Parse(strings.NewReader(string(b))) }
