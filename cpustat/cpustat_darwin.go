package cpustat

import "unsafe"
import "time"
import "math"
import "github.com/measure/metrics"
import "github.com/measure/os/misc"

// TODO: Per CPU stats - are they available?

/*
#include <mach/mach_init.h>
#include <mach/mach_error.h>
#include <mach/mach_host.h>
#include <mach/mach_port.h>
#include <mach/host_info.h>
*/
import "C"

// CPUStat encapsulates cpu performance information per cpu
type CPUStat struct {
	All *PerCPU
	m   *metrics.MetricContext
}

// PerCPU encapsulates cpu performance for a particular
// CPU
type PerCPU struct {
	User        *metrics.Counter
	UserLowPrio *metrics.Counter
	System      *metrics.Counter
	Idle        *metrics.Counter
	Total       *metrics.Counter // total ticks
}

// New returns an instance of CPUStat and starts collecting
// metrics at specified step
// TODO: add an argument or step of -1 should override automatic
// collection
func New(m *metrics.MetricContext, Step time.Duration) *CPUStat {
	c := new(CPUStat)
	c.All = PerCPUNew(m, "cpu")
	c.m = m
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()
	return c
}

// Collect starts collecting metris - usually called from New
func (s *CPUStat) Collect() {

	// collect CPU stats for All cpus aggregated
	var cpuinfo C.host_cpu_load_info_data_t
	count := C.mach_msg_type_number_t(C.HOST_CPU_LOAD_INFO_COUNT)
	host := C.mach_host_self()

	ret := C.host_statistics(C.host_t(host), C.HOST_CPU_LOAD_INFO,
		C.host_info_t(unsafe.Pointer(&cpuinfo)), &count)

	if ret != C.KERN_SUCCESS {
		return
	}

	s.All.User.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_USER]))
	s.All.UserLowPrio.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_NICE]))
	s.All.System.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_SYSTEM]))
	s.All.Idle.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_IDLE]))

	s.All.Total.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_USER]) +
		uint64(cpuinfo.cpu_ticks[C.CPU_STATE_SYSTEM]) +
		uint64(cpuinfo.cpu_ticks[C.CPU_STATE_NICE]) +
		uint64(cpuinfo.cpu_ticks[C.CPU_STATE_IDLE]))

}

// Usage returns current total CPU usage in percentage across all CPUs
func (s *CPUStat) Usage() float64 {
	return s.All.Usage()
}

// UserSpace returns time spent in userspace as percentage across all
// CPUs
func (s *CPUStat) UserSpace() float64 {
	return s.All.UserSpace()
}

// Kernel returns time spent in userspace as percentage across all
// CPUs
func (s *CPUStat) Kernel() float64 {
	return s.All.Kernel()
}

// PerCPUNew returns a struct representing counters for
// per CPU statistics
func PerCPUNew(m *metrics.MetricContext, cpu string) *PerCPU {
	o := new(PerCPU)
	// initialize metrics and register
	// XXX: need to adopt it to similar to linux and pass
	// cpu name as argument when we are collecting per cpu
	// information
	misc.InitializeMetrics(o, m, "cpustat.cpu", true)
	return o
}

// Usage returns total percentage of CPU used
func (o *PerCPU) Usage() float64 {
	u := o.User.ComputeRate()
	n := o.UserLowPrio.ComputeRate()
	s := o.System.ComputeRate()
	t := o.Total.ComputeRate()

	if u != math.NaN() && s != math.NaN() && t != math.NaN() && t > 0 {
		return (u + s + n) / t * 100
	}
	return math.NaN()
}

// UserSpace returns percentage of time spent in userspace
// on this CPU
func (o *PerCPU) UserSpace() float64 {
	u := o.User.ComputeRate()
	n := o.UserLowPrio.ComputeRate()
	t := o.Total.ComputeRate()
	if u != math.NaN() && t != math.NaN() && n != math.NaN() && t > 0 {
		return (u + n) / t * 100
	}
	return math.NaN()
}

// Kernel returns percentage of time spent in kernel
// on this CPU
func (o *PerCPU) Kernel() float64 {
	s := o.System.ComputeRate()
	t := o.Total.ComputeRate()
	if s != math.NaN() && t != math.NaN() && t > 0 {
		return (s / t) * 100
	}
	return math.NaN()
}
