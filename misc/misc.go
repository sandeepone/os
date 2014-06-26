// Copyright (c) 2014 Square, Inc

package misc

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/measure/metrics"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Interface interface{}

func ParseUint(in string) uint64 {
	out, err := strconv.ParseUint(in, 10, 64) // decimal, 64bit
	if err != nil {
		return 0
	}
	return out
}

func ReadUintFromFile(path string) uint64 {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return 0
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		return ParseUint(scanner.Text())
	}
	return 0
}

func InitializeMetrics(c Interface, m *metrics.MetricContext, prefix string, register bool) {
	s := reflect.ValueOf(c).Elem()
	typeOfT := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if f.Kind().String() != "ptr" {
			continue
		}
		if f.Type().Elem() == reflect.TypeOf(metrics.Gauge{}) {
			name := prefix + "." + typeOfT.Field(i).Name
			g := metrics.NewGauge()
			if register {
				m.Register(g, name)
			}
			f.Set(reflect.ValueOf(g))
		}
		if f.Type().Elem() == reflect.TypeOf(metrics.Counter{}) {
			name := prefix + "." + typeOfT.Field(i).Name
			g := metrics.NewCounter()
			if register {
				m.Register(g, name)
			}
			f.Set(reflect.ValueOf(g))
		}
	}
	return
}

// move these to cgroup library
// discover where memory subsystem is mounted

func FindCgroupMount(subsystem string) (string, error) {

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := regexp.MustCompile("[\\s]+").Split(scanner.Text(), 6)
		if f[2] == "cgroup" {
			for _, o := range strings.Split(f[3], ",") {
				if o == subsystem {
					return f[1], nil
				}
			}
		}
	}

	return "", errors.New("no cgroup mount found")
}

func FindCgroups(mountpoint string) ([]string, error) {
	cgroups := make([]string, 0, 128)

	_ = filepath.Walk(
		mountpoint,
		func(path string, f os.FileInfo, _ error) error {
			if f.IsDir() && path != mountpoint {
				// skip cgroups with no tasks
				dat, err := ioutil.ReadFile(path + "/" + "tasks")
				if err == nil && len(dat) > 0 {
					cgroups = append(cgroups, path)
				}
			}
			return nil
		})

	return cgroups, nil
}

type ByteSize float64

const (
	_           = iota
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)

func (b ByteSize) String() string {
	switch {
	case b >= YB:
		return fmt.Sprintf("%.2fYB", b/YB)
	case b >= ZB:
		return fmt.Sprintf("%.2fZB", b/ZB)
	case b >= EB:
		return fmt.Sprintf("%.2fEB", b/EB)
	case b >= PB:
		return fmt.Sprintf("%.2fPB", b/PB)
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.2fB", b)
}

type BitSize float64

const (
	_          = iota
	Kb BitSize = 1 << (10 * iota)
	Mb
	Gb
	Tb
	Pb
	Eb
	Zb
	Yb
)

func (b BitSize) String() string {
	switch {
	case b >= Yb:
		return fmt.Sprintf("%.2fYb", b/Yb)
	case b >= Zb:
		return fmt.Sprintf("%.2fZb", b/Zb)
	case b >= Eb:
		return fmt.Sprintf("%.2fEb", b/Eb)
	case b >= Pb:
		return fmt.Sprintf("%.2fPb", b/Pb)
	case b >= Tb:
		return fmt.Sprintf("%.2fTb", b/Tb)
	case b >= Gb:
		return fmt.Sprintf("%.2fGb", b/Gb)
	case b >= Mb:
		return fmt.Sprintf("%.2fMb", b/Mb)
	case b >= Kb:
		return fmt.Sprintf("%.2fKb", b/Kb)
	}
	return fmt.Sprintf("%.2fb", b)
}

const (
	CGROUP_BLKIO uint8 = iota
	CGROUP_CPU
	CGROUP_CPUACCT
	CGROUP_CPUSET
	CGROUP_DEVICES
	CGROUP_FREEZER
	CGROUP_MEMORY
	CGROUP_NET_CLS
	CGROUP_NS
)
