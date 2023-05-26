package memcached

import (
	"testing"
	"time"
)

func TestFuncStat(t *testing.T) {
	stat := &FuncStat{func() string { return "lol" }}
	if stat.String() != "lol" {
		t.Error("Should be 'lol'", stat.String())
	}
}

func TestTimerStat(t *testing.T) {
	stat := NewTimerStat()
	time.Sleep(time.Duration(1) * time.Second)
	if stat.String() != "1" {
		t.Error("Should be '1'", stat.String())
	}
}
