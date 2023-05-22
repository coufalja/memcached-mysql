//go:build !windows

package memcached

import "syscall"

func getRusage(usage usageType) float64 {
	rusage := &syscall.Rusage{}
	syscall.Getrusage(0, rusage)
	var time *syscall.Timeval
	if usage == UserTime {
		time = &rusage.Utime
	} else {
		time = &rusage.Stime
	}
	nsec := time.Nano()
	return float64(nsec) / 1000000000
}
