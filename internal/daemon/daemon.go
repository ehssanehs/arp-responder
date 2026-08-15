package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/ehssanehs/arp-responder/internal/config"
	"github.com/ehssanehs/arp-responder/internal/metrics"
	"github.com/ehssanehs/arp-responder/internal/network"
	"github.com/ehssanehs/arp-responder/internal/targets"
)

type Loader struct { ConfigPath, TargetsPath string; engine *network.Engine; metrics *metrics.Metrics; log *slog.Logger; mu sync.Mutex; lastConfig, lastTargets []byte }
func NewLoader(cp,tp string,e *network.Engine,m *metrics.Metrics,l *slog.Logger)*Loader{return &Loader{ConfigPath:cp,TargetsPath:tp,engine:e,metrics:m,log:l}}
func (l *Loader) Load(force bool)(config.Config,error){
	l.mu.Lock(); defer l.mu.Unlock()
	cb,err:=os.ReadFile(l.ConfigPath); if err!=nil{return config.Config{},fmt.Errorf("read config: %w",err)}
	tb,err:=os.ReadFile(l.TargetsPath); if err!=nil{return config.Config{},fmt.Errorf("read targets: %w",err)}
	if !force && string(cb)==string(l.lastConfig) && string(tb)==string(l.lastTargets) { c,e:=config.Load(l.ConfigPath); return c,e }
	c,err:=config.Load(l.ConfigPath); if err!=nil{return c,err}
	set,err:=targets.ParseBytes(tb); if err!=nil{return c,fmt.Errorf("parse targets: %w",err)}
	runtime := &network.RuntimeConfig{Interfaces:c.Interfaces,ReplyMAC:c.ReplyMAC,Targets:set}
	if force { err=l.engine.Initialize(runtime) } else { err=l.engine.Update(runtime) }; if err!=nil{return c,err}
	l.lastConfig=append(l.lastConfig[:0],cb...);l.lastTargets=append(l.lastTargets[:0],tb...)
	if !force { l.metrics.Reloads.Inc(); l.log.Info("configuration reloaded","target_ranges",set.LenRanges()) }
	return c,nil
}
func (l *Loader) RunEngine(ctx context.Context) error { go func(){<-ctx.Done();l.engine.Stop()}(); return l.engine.Run() }
