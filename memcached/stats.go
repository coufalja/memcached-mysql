package memcached

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Stats map[string]fmt.Stringer

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
	USER_TIME usageType = iota
	SYSTEM_TIME
)

func getRusage(usage usageType) float64 {
	rusage := &syscall.Rusage{}
	syscall.Getrusage(0, rusage)
	var time *syscall.Timeval
	if usage == USER_TIME {
		time = &rusage.Utime
	} else {
		time = &rusage.Stime
	}
	nsec := time.Nano()
	return float64(nsec) / 1000000000
}

func NewStats() Stats {
	s := make(Stats)
	s["pid"] = &StaticStat{strconv.Itoa(os.Getpid())}
	s["uptime"] = NewTimerStat()
	s["time"] = &FuncStat{func() string { return strconv.Itoa(int(time.Now().Unix())) }}
	s["version"] = &StaticStat{VERSION}
	s["golang"] = &StaticStat{runtime.Version()}
	s["goroutines"] = &FuncStat{func() string { return strconv.Itoa(runtime.NumGoroutine()) }}
	s["rusage_user"] = &FuncStat{func() string { return fmt.Sprintf("%f", getRusage(USER_TIME)) }}
	s["rusage_system"] = &FuncStat{func() string { return fmt.Sprintf("%f", getRusage(SYSTEM_TIME)) }}
	s["cmd_get"] = NewCounterStat()
	s["cmd_set"] = NewCounterStat()
	s["get_hits"] = NewCounterStat()
	s["get_misses"] = NewCounterStat()
	s["curr_connections"] = NewCounterStat()
	s["total_connections"] = NewCounterStat()
	s["evictions"] = NewCounterStat()
	return s
}
