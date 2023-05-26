package memcached

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

type TimerStat struct {
	Start int64
}

func (t *TimerStat) String() string {
	return strconv.Itoa(int(time.Now().Unix() - t.Start))
}

func NewTimerStat() *TimerStat {
	return &TimerStat{time.Now().Unix()}
}

type FuncStat struct {
	Callable func() string
}

func (f *FuncStat) String() string {
	return f.Callable()
}

type usageType int

const (
	UserTime usageType = iota
	SystemTime
)

type Stats struct {
	PID              string
	Uptime           *TimerStat
	Time             *FuncStat
	Version          string
	Golang           string
	Goroutines       *FuncStat
	RUsageUser       *FuncStat
	RUsageSystem     *FuncStat
	CMDGet           *atomic.Int64
	CMDSet           *atomic.Int64
	GetHits          *atomic.Int64
	GetMisses        *atomic.Int64
	CurrConnections  *atomic.Int64
	TotalConnections *atomic.Int64
	Evictions        *atomic.Int64
}

func (s Stats) Snapshot() map[string]string {
	m := make(map[string]string)
	m["pid"] = s.PID
	m["uptime"] = s.Uptime.String()
	m["time"] = s.Time.String()
	m["version"] = s.Version
	m["golang"] = s.Golang
	m["goroutines"] = s.Goroutines.String()
	m["rusage_user"] = s.RUsageUser.String()
	m["rusage_system"] = s.RUsageSystem.String()
	m["cmd_get"] = atomicToString(s.CMDGet)
	m["cmd_set"] = atomicToString(s.CMDSet)
	m["get_hits"] = atomicToString(s.GetHits)
	m["get_misses"] = atomicToString(s.GetMisses)
	m["curr_connections"] = atomicToString(s.CurrConnections)
	m["total_connections"] = atomicToString(s.TotalConnections)
	m["evictions"] = atomicToString(s.Evictions)
	return m
}

func atomicToString(a *atomic.Int64) string {
	return strconv.Itoa(int(a.Load()))
}

func NewStats() Stats {
	s := Stats{}
	s.PID = strconv.Itoa(os.Getpid())
	s.Uptime = NewTimerStat()
	s.Time = &FuncStat{func() string { return strconv.Itoa(int(time.Now().Unix())) }}
	s.Version = VERSION
	s.Golang = runtime.Version()
	s.Goroutines = &FuncStat{func() string { return strconv.Itoa(runtime.NumGoroutine()) }}
	s.RUsageUser = &FuncStat{func() string { return fmt.Sprintf("%f", getRusage(UserTime)) }}
	s.RUsageSystem = &FuncStat{func() string { return fmt.Sprintf("%f", getRusage(SystemTime)) }}
	s.CMDGet = &atomic.Int64{}
	s.CMDSet = &atomic.Int64{}
	s.GetHits = &atomic.Int64{}
	s.GetMisses = &atomic.Int64{}
	s.CurrConnections = &atomic.Int64{}
	s.TotalConnections = &atomic.Int64{}
	s.Evictions = &atomic.Int64{}

	return s
}
