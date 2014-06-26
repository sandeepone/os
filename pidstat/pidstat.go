package pidstat

import (
	"math"
	"sort"
)

// ProcessStatInterface defines common methods that all
// platform specific ProcessStat type must implement

type ProcessStatInterface interface {
	ByCPUUsage() []*PerProcessStat
	ByMemUsage() []*PerProcessStat
	SetPidFilter(PidFilterFunc)
}

var _ ProcessStatInterface = &ProcessStat{}

// PerProcessStatInterface defines common methods that
// all platform specific PerProcessStat types must
// implement

type PerProcessStatInterface interface {
	CPUUsage() float64
	MemUsage() float64
}

var _ PerProcessStatInterface = &PerProcessStat{}

// ByCPUUsage implements sort.Interface for []*PerProcessStat based on
// the Usage() method
type ByCPUUsage []*PerProcessStat

func (a ByCPUUsage) Len() int           { return len(a) }
func (a ByCPUUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCPUUsage) Less(i, j int) bool { return a[i].CPUUsage() > a[j].CPUUsage() }

// ByCPUUsage() returns an slice of *PerProcessStat entries sorted
// by CPU usage
func (c *ProcessStat) ByCPUUsage() []*PerProcessStat {
	v := make([]*PerProcessStat, 0)
	for _, o := range c.Processes {
		if !math.IsNaN(o.CPUUsage()) {
			v = append(v, o)
		}
	}
	sort.Sort(ByCPUUsage(v))
	return v
}

type ByMemUsage []*PerProcessStat

func (a ByMemUsage) Len() int           { return len(a) }
func (a ByMemUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByMemUsage) Less(i, j int) bool { return a[i].MemUsage() > a[j].MemUsage() }

// ByMemUsage() returns an slice of *PerProcessStat entries sorted
// by Memory usage
func (c *ProcessStat) ByMemUsage() []*PerProcessStat {
	v := make([]*PerProcessStat, 0)
	for _, o := range c.Processes {
		if !math.IsNaN(o.MemUsage()) {
			v = append(v, o)
		}
	}
	sort.Sort(ByMemUsage(v))
	return v
}

type PidFilterFunc func(pidstat *PerProcessStat) (interested bool)

func (f PidFilterFunc) Filter(pidstat *PerProcessStat) (interested bool) {
	return f(pidstat)
}

func defaultPidFilter(pidstat *PerProcessStat) bool {
	return true
}
