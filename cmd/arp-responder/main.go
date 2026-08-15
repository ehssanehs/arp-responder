package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/ehssanehs/arp-responder/internal/targets"

	"github.com/ehssanehs/arp-responder/internal/config"
	"github.com/ehssanehs/arp-responder/internal/daemon"
	applog "github.com/ehssanehs/arp-responder/internal/logging"
	appmetrics "github.com/ehssanehs/arp-responder/internal/metrics"
	"github.com/ehssanehs/arp-responder/internal/network"
	"github.com/ehssanehs/arp-responder/internal/reload"
)

func main(){
	configPath:=flag.String("config","/etc/arp-responder/config.yaml","configuration file")
	targetsPath:=flag.String("targets","/etc/arp-responder/targets.conf","target addresses file")
	check:=flag.Bool("check",false,"validate configuration and exit")
	flag.Parse()
	if err:=run(*configPath,*targetsPath,*check);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
}
func run(configPath,targetsPath string,check bool)(err error){
	defer func(){if v:=recover();v!=nil{err=fmt.Errorf("unexpected panic: %v\n%s",v,debug.Stack())}}()
	cfg,err:=config.Load(configPath);if err!=nil{return fmt.Errorf("configuration: %w",err)}
	if check {
		f, openErr := os.Open(targetsPath); if openErr != nil { return fmt.Errorf("targets: %w", openErr) }; defer f.Close()
		if _, parseErr := targets.Parse(f); parseErr != nil { return fmt.Errorf("targets: %w", parseErr) }
		return nil
	}
	log,closer,err:=applog.New(cfg);if err!=nil{return err};if closer!=nil{defer closer.Close()}
	m:=appmetrics.New(); engine,err:=network.New(log,m);if err!=nil{return err}
	loader:=daemon.NewLoader(configPath,targetsPath,engine,m,log)
	if _,err=loader.Load(true);err!=nil{return fmt.Errorf("initialization: %w",err)}
	ctx,cancel:=signal.NotifyContext(context.Background(),syscall.SIGTERM,syscall.SIGINT);defer cancel()
	var server *http.Server
	if cfg.Metrics.Enabled {
		server=&http.Server{Addr:cfg.Metrics.Listen,Handler:m.Handler(),ReadHeaderTimeout:5*time.Second}
		go func(){log.Info("metrics endpoint started","listen",cfg.Metrics.Listen);if e:=server.ListenAndServe();e!=nil&&!errors.Is(e,http.ErrServerClosed){log.Error("metrics server failed","error",e);cancel()}}()
	}
	watch:=reload.New([]string{configPath,targetsPath},cfg.ReloadInterval(),log)
	go func(){if e:=watch.Run(ctx,func()error{_,e:=loader.Load(false);return e});e!=nil{log.Error("reloader stopped","error",e);cancel()}}()
	log.Info("ARP responder started","interfaces",cfg.Interfaces)
	err=loader.RunEngine(ctx)
	if server!=nil{shutdownCtx,c:=context.WithTimeout(context.Background(),5*time.Second);_ = server.Shutdown(shutdownCtx);c()}
	log.Info("ARP responder stopped")
	return err
}

