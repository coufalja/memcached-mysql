package memcached

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaticStat(t *testing.T) {
	stat := &StaticStat{"lol"}
	if stat.String() != "lol" {
		t.Error("Should be 'lol'", stat.String())
	}
}

func TestFuncStat(t *testing.T) {
	stat := &FuncStat{func() string { return "lol" }}
	if stat.String() != "lol" {
		t.Error("Should be 'lol'", stat.String())
	}
}

func TestCounterStat(t *testing.T) {
	stat := NewCounterStat()
	var i int
	for i = 0; i < 10; i++ {
		stat.Increment(1)
	}
	require.Eventually(t, func() bool { return stat.String() == "10" }, 10*time.Second, 10*time.Millisecond)
	for i = 0; i < 10; i++ {
		stat.Decrement(1)
	}
	require.Eventually(t, func() bool { return stat.String() == "0" }, 10*time.Second, 10*time.Millisecond)
	stat.SetCount(100)
	if stat.String() != "100" {
		t.Error("Should be '100'", stat.String())
	}
}

func TestTimerStat(t *testing.T) {
	stat := NewTimerStat()
	time.Sleep(time.Duration(1) * time.Second)
	if stat.String() != "1" {
		t.Error("Should be '1'", stat.String())
	}
}
