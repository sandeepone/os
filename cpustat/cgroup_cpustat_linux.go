// Copyright (c) 2014 Square, Inc

package cpustat

/*
Package cpustat implements collection of cpu performance
metrics
*/

import (
	"bufio"
	"github.com/measure/metrics"
	"github.com/measure/os/misc"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// CgroupStat encapsulates cpu information about various
// cgroups and provides functions to collect
type CgroupStat struct {
	Cgroups    map[string]*PerCgroupStat
	m          *metrics.MetricContext
	Mountpoint string
}

// NewCgroupStat returns an instance of CgroupStat
func NewCgroupStat(m *metrics.MetricContext, Step time.Duration) *CgroupStat {
	c := new(CgroupStat)
	c.m = m

	c.Cgroups = make(map[string]*PerCgroupStat, 1)

	mountpoint, err := misc.FindCgroupMount("cpu")
	if err != nil {
		return c
	}
	c.Mountpoint = mountpoint

	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect(mountpoint)
		}
	}()

	return c
}

// Collect starts collecting metrics
func (c *CgroupStat) Collect(mountpoint string) {

	cgroups, err := misc.FindCgroups(mountpoint)
	if err != nil {
		return
	}

	// stop tracking cgroups which don't exist
	// anymore or have no tasks
	cgroupsMap := make(map[string]bool, len(cgroups))
	for _, cgroup := range cgroups {
		cgroupsMap[cgroup] = true
	}

	for cgroup := range c.Cgroups {
		_, ok := cgroupsMap[cgroup]
		if !ok {
			delete(c.Cgroups, cgroup)
		}
	}

	for _, cgroup := range cgroups {
		_, ok := c.Cgroups[cgroup]
		if !ok {
			c.Cgroups[cgroup] = NewPerCgroupStat(c.m, cgroup, mountpoint)
		}
		c.Cgroups[cgroup].Metrics.Collect()
	}
}

// PerCgroupStat encapsulates cpu information about a single
// cgroup
type PerCgroupStat struct {
	Metrics *PerCgroupStatMetrics
	m       *metrics.MetricContext
}

// NewPerCgroupStat returns an instance of PerCgroupStat
func NewPerCgroupStat(m *metrics.MetricContext, path string, mp string) *PerCgroupStat {
	c := new(PerCgroupStat)
	c.m = m

	c.Metrics = NewPerCgroupStatMetrics(m, path, mp)

	return c
}

// Throttle returns as percentage of time that
// the cgroup couldn't get enough cpu
// rate ((nr_throttled * period) / quota)
// XXX: add support for real-time scheduler stats
func (s *PerCgroupStat) Throttle() float64 {
	o := s.Metrics
	throttledSec := o.ThrottledTime.ComputeRate()

	return (throttledSec / (1 * 1000 * 1000 * 1000)) * 100
}

// Quota returns how many logical CPUs can be used
func (s *PerCgroupStat) Quota() float64 {
	o := s.Metrics
	return (o.CfsQuotaUs.Get() / o.CfsPeriodUs.Get())
}

// PerCgroupStatMetrics encapsulates cgroup cpu metrics
// per cgroup
type PerCgroupStatMetrics struct {
	NrPeriods     *metrics.Counter
	NrThrottled   *metrics.Counter
	ThrottledTime *metrics.Counter
	CfsPeriodUs   *metrics.Gauge
	CfsQuotaUs    *metrics.Gauge
	path          string
}

// NewPerCgroupStatMetrics returns an instance of per-cgroup
// metrics
func NewPerCgroupStatMetrics(m *metrics.MetricContext, path string, mp string) *PerCgroupStatMetrics {
	c := new(PerCgroupStatMetrics)
	c.path = path

	// initialize all metrics and register them
	prefix, _ := filepath.Rel(mp, path)
	misc.InitializeMetrics(c, m, "cpustat.cgroup."+prefix, true)

	return c
}

// Collect scans cgroup mountpoint and populates cgroup
// cpu metrics
func (s *PerCgroupStatMetrics) Collect() {
	file, err := os.Open(s.path + "/" + "cpu.stat")
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := regexp.MustCompile("\\s+").Split(scanner.Text(), 2)

		if f[0] == "nr_periods" {
			s.NrPeriods.Set(misc.ParseUint(f[1]))
		}

		if f[0] == "nr_throttled" {
			s.NrThrottled.Set(misc.ParseUint(f[1]))
		}

		if f[0] == "throttled_time" {
			s.ThrottledTime.Set(misc.ParseUint(f[1]))
		}
	}

	s.CfsPeriodUs.Set(
		float64(misc.ReadUintFromFile(
			s.path + "/" + "cpu.cfs_period_us")))

	s.CfsQuotaUs.Set(
		float64(misc.ReadUintFromFile(
			s.path + "/" + "cpu.cfs_quota_us")))
}
