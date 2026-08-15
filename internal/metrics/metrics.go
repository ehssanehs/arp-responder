package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// Counter is an allocation-free monotonic counter.
type Counter struct{ value atomic.Uint64 }

func (c *Counter) Inc()         { c.value.Add(1) }
func (c *Counter) Value() uint64 { return c.value.Load() }

type Metrics struct {
	Requests Counter
	Replies  Counter
	Invalid  Counter
	Dropped  Counter
	Reloads  Counter
}

func New() *Metrics { return &Metrics{} }

func (m *Metrics) Handler() http.Handler { return http.HandlerFunc(m.serveHTTP) }

func (m *Metrics) serveHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	writeCounter(w, "arp_responder_requests_total", "Valid ARP requests received.", m.Requests.Value())
	writeCounter(w, "arp_responder_replies_total", "ARP replies sent.", m.Replies.Value())
	writeCounter(w, "arp_responder_invalid_packets_total", "Malformed or ignored packets.", m.Invalid.Value())
	writeCounter(w, "arp_responder_dropped_packets_total", "Matched requests that could not be sent.", m.Dropped.Value())
	writeCounter(w, "arp_responder_reloads_total", "Successful configuration reloads.", m.Reloads.Value())

	var usage unix.Rusage
	if unix.Getrusage(unix.RUSAGE_SELF, &usage) == nil {
		cpu := timevalSeconds(usage.Utime) + timevalSeconds(usage.Stime)
		fmt.Fprintln(w, "# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.")
		fmt.Fprintln(w, "# TYPE process_cpu_seconds_total counter")
		fmt.Fprintf(w, "process_cpu_seconds_total %.6f\n", cpu)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	fmt.Fprintln(w, "# HELP process_resident_memory_bytes Resident memory approximation reported by the Go runtime.")
	fmt.Fprintln(w, "# TYPE process_resident_memory_bytes gauge")
	fmt.Fprintf(w, "process_resident_memory_bytes %d\n", memory.Sys)
	fmt.Fprintln(w, "# HELP go_goroutines Number of goroutines that currently exist.")
	fmt.Fprintln(w, "# TYPE go_goroutines gauge")
	fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())
}

func writeCounter(w http.ResponseWriter, name, help string, value uint64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
}

func timevalSeconds(t unix.Timeval) float64 {
	return float64(t.Sec) + float64(t.Usec)/1e6
}
