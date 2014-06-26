// Copyright (c) 2014 Square, Inc

package pidstat

import (
	"fmt"
	"github.com/measure/os/misc"
	"github.com/measure/metrics"
	"os/user"
	"reflect"
	"time"
	"unsafe"
)

// https://developer.apple.com/library/Mac/qa/qa1398/_index.html

/*
#include <mach/mach.h>
#include <mach/task_info.h>
#include <mach/mach_time.h>
#include <sys/sysctl.h>


int get_process_info(struct kinfo_proc *kp, pid_t pid)
{
	size_t len = sizeof(struct kinfo_proc);
	static int name[] = { CTL_KERN, KERN_PROC, KERN_PROC_PID, 0 };
	name[3] = pid;
	kp->kp_proc.p_comm[0] = '\0'; // jic
	return sysctl((int *)name, sizeof(name)/sizeof(*name), kp, &len, NULL, 0);
}
uint64_t absolute_to_nano(uint64_t abs)
{
	static mach_timebase_info_data_t s_timebase_info;

	if (s_timebase_info.denom == 0) {
		(void) mach_timebase_info(&s_timebase_info);
	}

        return (uint64_t)((abs * s_timebase_info.numer) / (s_timebase_info.denom));
}
*/
import "C"

const NS = 1 * 1000 * 1000 * 1000

type ProcessStat struct {
	Processes map[string]*PerProcessStat
	m         *metrics.MetricContext
	hport     C.host_t
}

// NewProcessStat allocates a new ProcessStat object
// Arguments:
// m - *metricContext

// Collects metrics every Step seconds
// Drops refresh interval by Step for every additional
// 1024 processes
// TODO: Implement better heuristics to manage load
//   * Collect metrics for newer processes at faster rate
//   * Slower rate for processes with neglible rate?

func NewProcessStat(m *metrics.MetricContext, Step time.Duration) *ProcessStat {
	c := new(ProcessStat)
	c.m = m

	c.Processes = make(map[string]*PerProcessStat, 1024)
	c.hport = C.host_t(C.mach_host_self())

	var n int
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			p := int(len(c.Processes) / 1024)
			if n == 0 {
				c.Collect(true)
			}
			// always collect all metrics for first two samples
			// and if number of processes < 1024
			if p < 1 || n%p == 0 {
				c.Collect(false)
			}
			n++
		}
	}()

	return c
}

// not implemented on darwin
func (s *ProcessStat) SetPidFilter(filter PidFilterFunc) {
	return
}

// reference /usr/include/mach/task_info.h
// works on MacOSX 10.9.2; YMMV might vary

func (c *ProcessStat) Collect(collectAttributes bool) {

	h := c.Processes
	for _, v := range h {
		v.dead = true
	}

	var pDefaultSet C.processor_set_name_t
	var pDefaultSetControl C.processor_set_t
	var tasks C.task_array_t
	var taskCount C.mach_msg_type_number_t

	if C.processor_set_default(c.hport, &pDefaultSet) != C.KERN_SUCCESS {
		return
	}

	// get privileged port to get information about all tasks

	if C.host_processor_set_priv(C.host_priv_t(c.hport),
		pDefaultSet, &pDefaultSetControl) != C.KERN_SUCCESS {
		return
	}

	if C.processor_set_tasks(pDefaultSetControl, &tasks, &taskCount) != C.KERN_SUCCESS {
		return
	}

	// convert tasks to a Go slice
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(tasks)),
		Len:  int(taskCount),
		Cap:  int(taskCount),
	}

	goTaskList := *(*[]C.task_name_t)(unsafe.Pointer(&hdr))

	// mach_msg_type_number_t - type natural_t = uint32_t
	var i uint32
	for i = 0; i < uint32(taskCount); i++ {

		taskId := goTaskList[i]
		var pid C.int
		// var tinfo C.task_info_data_t
		var count C.mach_msg_type_number_t
		var taskBasicInfo C.mach_task_basic_info_data_t
		var taskAbsoluteInfo C.task_absolutetime_info_data_t

		if (C.pid_for_task(C.mach_port_name_t(taskId), &pid) != C.KERN_SUCCESS) ||
			(pid < 0) {
			continue
		}

		count = C.MACH_TASK_BASIC_INFO_COUNT
		kr := C.task_info(taskId, C.MACH_TASK_BASIC_INFO,
			(C.task_info_t)(unsafe.Pointer(&taskBasicInfo)),
			&count)
		if kr != C.KERN_SUCCESS {
			continue
		}

		spid := fmt.Sprintf("%v", pid)
		pidstat, ok := h[spid]
		if !ok {
			pidstat = NewPerProcessStat(c.m, spid)
			h[spid] = pidstat
		}

		if collectAttributes || !ok {
			pidstat.CollectAttributes(pid)
		}

		pidstat.Metrics.VirtualSize.Set(float64(taskBasicInfo.virtual_size))
		pidstat.Metrics.ResidentSize.Set(float64(taskBasicInfo.resident_size))
		pidstat.Metrics.ResidentSizeMax.Set(float64(taskBasicInfo.resident_size_max))

		count = C.TASK_ABSOLUTETIME_INFO_COUNT
		kr = C.task_info(taskId, C.TASK_ABSOLUTETIME_INFO,
			(C.task_info_t)(unsafe.Pointer(&taskAbsoluteInfo)),
			&count)
		if kr != C.KERN_SUCCESS {
			continue
		}
		pidstat.Metrics.UserTime.Set(
			uint64(C.absolute_to_nano(taskAbsoluteInfo.total_user)))
		pidstat.Metrics.SystemTime.Set(
			uint64(C.absolute_to_nano(taskAbsoluteInfo.total_system)))
		pidstat.dead = false
	}

	// remove dead processes
	for k, v := range h {
		if v.dead {
			delete(h, k)
		}
	}

}

// Per Process functions
type PerProcessStat struct {
	pid     string
	Uid     int
	user    string
	comm    string
	Metrics *PerProcessStatMetrics
	m       *metrics.MetricContext
	dead    bool
}

func NewPerProcessStat(m *metrics.MetricContext, p string) *PerProcessStat {
	c := new(PerProcessStat)
	c.m = m
	c.pid = p
	c.Metrics = NewPerProcessStatMetrics(m, p)
	return c
}

// CPUUsage() returns current cpu usage percent, user+system for process
func (s *PerProcessStat) CPUUsage() float64 {
	o := s.Metrics
	rate_ns := o.UserTime.ComputeRate() + o.SystemTime.ComputeRate()
	return (rate_ns / float64(NS)) * 100
}

func (s *PerProcessStat) MemUsage() float64 {
	o := s.Metrics
	return o.ResidentSize.Get()
}

func (s *PerProcessStat) Pid() string {
	return s.pid
}

func (s *PerProcessStat) Comm() string {
	return s.comm
}

func (s *PerProcessStat) User() string {
	return s.user
}

type PerProcessStatMetrics struct {
	VirtualSize     *metrics.Gauge
	ResidentSize    *metrics.Gauge
	ResidentSizeMax *metrics.Gauge
	UserTime        *metrics.Counter
	SystemTime      *metrics.Counter
}

func NewPerProcessStatMetrics(m *metrics.MetricContext, pid string) *PerProcessStatMetrics {
	s := new(PerProcessStatMetrics)
	// initialize all metrics and do NOT register for now
	misc.InitializeMetrics(s, m, "pidstat.", false)
	return s
}

func (s *PerProcessStat) CollectAttributes(pid C.int) {
	// some cgo follows
	var kp C.struct_kinfo_proc

	C.get_process_info(&kp, C.pid_t(pid))
	s.comm = C.GoString((*C.char)(unsafe.Pointer(&kp.kp_proc.p_comm)))
	s.Uid = int(kp.kp_eproc.e_ucred.cr_uid)
	u, err := user.LookupId(fmt.Sprintf("%v", s.Uid))
	if err == nil {
		s.user = u.Username
	}
}
