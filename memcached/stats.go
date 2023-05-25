package memcached

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type StaticStat struct {
	Value string
}

func (s *StaticStat) String() string {
	return s.Value
}

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

type CounterStat struct {
	Count        int
	calculations chan int
	mutex        sync.Mutex
}

func (c *CounterStat) Increment(num int) {
	c.calculations <- num
}

func (c *CounterStat) SetCount(num int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Count = num
}

func (c *CounterStat) Decrement(num int) {
	c.calculations <- -num
}

func (c *CounterStat) String() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return strconv.Itoa(c.Count)
}

func (c *CounterStat) work() {
	for num := range c.calculations {
		c.mutex.Lock()
		c.Count = c.Count + num
		c.mutex.Unlock()
	}
}

func NewCounterStat() *CounterStat {
	c := &CounterStat{}
	c.calculations = make(chan int, 100)
	go c.work()
	return c
}

type usageType int

const (
	UserTime usageType = iota
	SystemTime
)

type Stats struct {
	PID              *StaticStat
	Uptime           *TimerStat
	Time             *FuncStat
	Version          *StaticStat
	Golang           *StaticStat
	Goroutines       *FuncStat
	RUsageUser       *FuncStat
	RUsageSystem     *FuncStat
	CMDGet           *CounterStat
	CMDSet           *CounterStat
	GetHits          *CounterStat
	GetMisses        *CounterStat
	CurrConnections  *CounterStat
	TotalConnections *CounterStat
	Evictions        *CounterStat
}

func (s Stats) Snapshot() map[string]string {
	m := make(map[string]string)
	m["pid"] = s.PID.String()
	m["uptime"] = s.Uptime.String()
	m["time"] = s.Time.String()
	m["version"] = s.Version.String()
	m["golang"] = s.Golang.String()
	m["goroutines"] = s.Goroutines.String()
	m["rusage_user"] = s.RUsageUser.String()
	m["rusage_system"] = s.RUsageSystem.String()
	m["cmd_get"] = s.CMDGet.String()
	m["cmd_set"] = s.CMDSet.String()
	m["get_hits"] = s.GetHits.String()
	m["get_misses"] = s.GetMisses.String()
	m["curr_connections"] = s.CurrConnections.String()
	m["total_connections"] = s.TotalConnections.String()
	m["evictions"] = s.Evictions.String()
	return m
}

func NewStats() Stats {
	s := Stats{}
	s.PID = &StaticStat{strconv.Itoa(os.Getpid())}
	s.Uptime = NewTimerStat()
	s.Time = &FuncStat{func() string { return strconv.Itoa(int(time.Now().Unix())) }}
	s.Version = &StaticStat{VERSION}
	s.Golang = &StaticStat{runtime.Version()}
	s.Goroutines = &FuncStat{func() string { return strconv.Itoa(runtime.NumGoroutine()) }}
	s.RUsageUser = &FuncStat{func() string { return fmt.Sprintf("%f", getRusage(UserTime)) }}
	s.RUsageSystem = &FuncStat{func() string { return fmt.Sprintf("%f", getRusage(SystemTime)) }}
	s.CMDGet = NewCounterStat()
	s.CMDSet = NewCounterStat()
	s.GetHits = NewCounterStat()
	s.GetMisses = NewCounterStat()
	s.CurrConnections = NewCounterStat()
	s.TotalConnections = NewCounterStat()
	s.Evictions = NewCounterStat()
	return s
}
