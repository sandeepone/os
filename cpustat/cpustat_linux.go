// Copyright (c) 2014 Square, Inc

package cpustat

import (
	"bufio"
	"math"
	"os"
	"regexp"
	"time"

	"github.com/measure/metrics"
	"github.com/measure/os/misc"
)

// CPUStat encapsulates metric information about all CPUs
type CPUStat struct {
	All          *PerCPU
	ProcsRunning *metrics.Counter
	ProcsBlocked *metrics.Counter
	cpus         map[string]*PerCPU
	m            *metrics.MetricContext
}

// PerCPU encapsulates metrics about individual CPU performance
// and also provides few summary statistics
type PerCPU struct {
	User        *metrics.Counter
	UserLowPrio *metrics.Counter
	System      *metrics.Counter
	Idle        *metrics.Counter
	Iowait      *metrics.Counter
	Irq         *metrics.Counter
	Softirq     *metrics.Counter
	Steal       *metrics.Counter
	Guest       *metrics.Counter
	Total       *metrics.Counter // total jiffies
	// Computed stats
	UserspacePct *metrics.Gauge
	KernelPct    *metrics.Gauge
	UsagePct     *metrics.Gauge
}

// New returns an instance of CPUStat
func New(m *metrics.MetricContext, Step time.Duration) *CPUStat {
	c := new(CPUStat)
	c.All = NewPerCPU(m, "cpu")
	c.m = m
	c.cpus = make(map[string]*PerCPU, 1)
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()
	return c
}

// Collect captures metrics for all cpus and also publishes few summary
// statistics
// XXX: break this up into two smaller functions
func (s *CPUStat) Collect() {
	file, err := os.Open("/proc/stat")
	defer file.Close()

	if err != nil {
		return
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := regexp.MustCompile("\\s+").Split(scanner.Text(), -1)

		isCPU, err := regexp.MatchString("^cpu\\d*", f[0])
		if err == nil && isCPU {
			if f[0] == "cpu" {
				parseCPUline(s.All, f)
				populateComputedStats(s.All)
			} else {
				perCPU, ok := s.cpus[f[0]]
				if !ok {
					perCPU = NewPerCPU(s.m, f[0])
					s.cpus[f[0]] = perCPU
				}
				parseCPUline(perCPU, f)
				populateComputedStats(perCPU)
			}
		}
	}
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

// CPUS returns all CPUS found as a slice of strings
func (s *CPUStat) CPUS() []string {
	ret := make([]string, 1)
	for k := range s.cpus {
		ret = append(ret, k)
	}

	return ret
}

// PerCPUStat returns per-CPU stats for argument "cpu"
func (s *CPUStat) PerCPUStat(cpu string) *PerCPU {
	return s.cpus[cpu]
}

// NewPerCPU returns a struct representing counters for
// per CPU statistics
func NewPerCPU(m *metrics.MetricContext, name string) *PerCPU {
	o := new(PerCPU)

	// initialize all metrics and register them
	misc.InitializeMetrics(o, m, "cpustat."+name, true)
	return o
}

// Usage returns total percentage of CPU used
func (o *PerCPU) Usage() float64 {
	u := o.User.ComputeRate()
	n := o.UserLowPrio.ComputeRate()
	s := o.System.ComputeRate()
	t := o.Total.ComputeRate()

	if u != math.NaN() && n != math.NaN() && s != math.NaN() &&
		t != math.NaN() && t > 0 {
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

// Unexported functions
func parseCPUline(s *PerCPU, f []string) {
	s.User.Set(misc.ParseUint(f[1]))
	s.UserLowPrio.Set(misc.ParseUint(f[2]))
	s.System.Set(misc.ParseUint(f[3]))
	s.Idle.Set(misc.ParseUint(f[4]))
	s.Iowait.Set(misc.ParseUint(f[5]))
	s.Irq.Set(misc.ParseUint(f[6]))
	s.Softirq.Set(misc.ParseUint(f[7]))
	s.Steal.Set(misc.ParseUint(f[8]))
	s.Guest.Set(misc.ParseUint(f[9]))
	s.Total.Set(s.User.Get() + s.UserLowPrio.Get() + s.System.Get() + s.Idle.Get())
}

func populateComputedStats(s *PerCPU) {
	s.UserspacePct.Set(s.UserSpace())
	s.KernelPct.Set(s.Kernel())
	s.UsagePct.Set(s.Usage())
}
