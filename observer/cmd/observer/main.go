
//go:build linux

package main

//go:generate bash -c "mkdir -p bpf && if [ ! -f bpf/vmlinux.h ]; then bpftool btf dump file /sys/kernel/btf/vmlinux format c > bpf/vmlinux.h; fi"
//go:generate bash -c "bpf2go -target bpfel -cc clang -cflags '-O2 -g -D__TARGET_ARCH_x86' TraceWrite ./bpf/trace_write.bpf.c -- -I./bpf"

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	targetPID = flag.Int("pid", 0, "target process PID (app)")
	listen    = flag.String("listen", ":9100", "Prometheus listen addr")
)

var (
	writeBytes  = prometheus.NewCounter(prometheus.CounterOpts{Name: "app_write_bytes_total", Help: "Bytes (sys_enter_write)."})
	writeEvents = prometheus.NewCounter(prometheus.CounterOpts{Name: "app_write_events_total", Help: "Events (sys_enter_write)."})

	tcpSendBytes  = prometheus.NewCounter(prometheus.CounterOpts{Name: "app_tcp_send_bytes_total", Help: "Bytes via kprobe tcp_sendmsg."})
	tcpSendEvents = prometheus.NewCounter(prometheus.CounterOpts{Name: "app_tcp_send_events_total", Help: "Events via kprobe tcp_sendmsg."})

	uprobedBytes  = prometheus.NewCounter(prometheus.CounterOpts{Name: "app_uprobe_bytes_total", Help: "Bytes via uprobe AppWrite."})
	uprobedCalls  = prometheus.NewCounter(prometheus.CounterOpts{Name: "app_uprobe_calls_total", Help: "Calls to AppWrite (CPS proxy)."})
)

func main() {
	flag.Parse()
	if *targetPID <= 0 { log.Fatalf("must provide -pid") }
	if err := rlimit.RemoveMemlock(); err != nil { log.Fatalf("rlimit: %v", err) }

	prometheus.MustRegister(writeBytes, writeEvents, tcpSendBytes, tcpSendEvents, uprobedBytes, uprobedCalls)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("serving Prometheus on %s", *listen)
		log.Fatal(http.ListenAndServe(*listen, nil))
	}()

	objs := TraceWriteObjects{}
	if err := LoadTraceWriteObjects(&objs, nil); err != nil { log.Fatalf("load objs: %v", err) }
	defer objs.Close()

	if objs.Bss != nil {
		objs.Bss.TargetPid = uint32(*targetPID)
		if err := objs.Bss.Put(); err != nil { log.Printf("warn: BSS put: %v", err) }
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_write", objs.TpSysEnterWrite, nil)
	if err != nil { log.Fatalf("tracepoint attach: %v", err) }
	defer tp.Close()

	kp, err := link.Kprobe("tcp_sendmsg", objs.KprobeTcpSendmsg, nil)
	if err != nil { log.Printf("warn: kprobe tcp_sendmsg: %v", err) } else { defer kp.Close() }

	bin := exePath(*targetPID)
	if bin != "" {
		up, err := link.Uprobe(bin, "AppWrite", objs.UprobeAppWrite, nil)
		if err != nil { log.Printf("warn: uprobe AppWrite: %v", err) } else { defer up.Close() }
	} else {
		log.Printf("warn: could not resolve app binary path for uprobe")
	}

	rl, err := ringbuf.NewReader(objs.Events)
	if err != nil { log.Fatalf("ringbuf open: %v", err) }
	defer rl.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			rec, err := rl.Read()
			if err != nil { log.Printf("ringbuf read: %v", err); return }
			if len(rec.RawSample) < 20 { continue }
			bytes := byteOrder.Uint64(rec.RawSample[8:16])
			kind := byteOrder.Uint32(rec.RawSample[16:20])
			switch kind {
			case 1:
				writeEvents.Inc(); writeBytes.Add(float64(bytes))
			case 2:
				tcpSendEvents.Inc(); tcpSendBytes.Add(float64(bytes))
			case 3:
				uprobedCalls.Inc(); uprobedBytes.Add(float64(bytes))
			}
		}
	}()

	<-sigs
	log.Printf("shutdown")
}

func exePath(pid int) string {
	p := fmt.Sprintf("/proc/%d/exe", pid)
	link, err := os.Readlink(p)
	if err == nil { return link }
	cmdline := fmt.Sprintf("/proc/%d/cmdline", pid)
	b, err := ioutil.ReadFile(cmdline)
	if err != nil { return "" }
	parts := strings.Split(string(b), "\x00")
	if len(parts) > 0 && parts[0] != "" {
		if filepath.IsAbs(parts[0]) { return parts[0] }
	}
	return ""
}
