package reload

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch monitors parent directories so atomic file replacements are detected.
type Watch struct { paths []string; interval time.Duration; log *slog.Logger }
func New(paths []string, interval time.Duration, log *slog.Logger) *Watch { return &Watch{paths:paths,interval:interval,log:log} }
func (w *Watch) Run(ctx context.Context, reload func() error) error {
	watcher,err:=fsnotify.NewWatcher(); if err!=nil { return err }; defer watcher.Close()
	dirs:=map[string]struct{}{}
	for _,p:=range w.paths { dirs[filepath.Dir(p)]=struct{}{} }
	for d:=range dirs { if err:=watcher.Add(d); err!=nil { w.log.Warn("file watch unavailable; periodic reload remains active","directory",d,"error",err) } }
	ticker:=time.NewTicker(w.interval); defer ticker.Stop()
	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done(): return nil
		case err:=<-watcher.Errors: if err!=nil { w.log.Warn("file watcher error","error",err) }
		case ev,ok:=<-watcher.Events:
			if !ok { continue }
			matched:=false
			for _,p:=range w.paths { if filepath.Clean(ev.Name)==filepath.Clean(p) { matched=true; break } }
			if matched { debounce=time.After(150*time.Millisecond) }
		case <-debounce:
			debounce=nil; if err:=reload(); err!=nil { w.log.Error("reload failed; retaining previous configuration","error",err) }
		case <-ticker.C:
			if err:=reload(); err!=nil { w.log.Error("periodic reload failed; retaining previous configuration","error",err) }
		}
	}
}
