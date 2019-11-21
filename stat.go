package main

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func GetNumberAndUnit(line string) (val float32, unit string) {
	eles := strings.Split(line, " ")

	unit = eles[len(eles)-1]
	v, _ := strconv.ParseFloat(eles[len(eles)-2], 32)
	val = float32(v)
	return
}

func UnitToWegith(unit string) (ret float32) {
	switch unit {
	case "kB":
		ret = 0.001
	case "MB":
		ret = 1
	}

	return
}

type Usage struct {
	Used  float32
	Total float32
}

func GetMemory() (ret Usage, reterr error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		reterr = err
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	totalLine, _, err := reader.ReadLine()
	_, _, err = reader.ReadLine()
	avLine, _, err := reader.ReadLine()

	total, tUnit := GetNumberAndUnit(string(totalLine))
	available, aUnit := GetNumberAndUnit(string(avLine))
	total, available = total*UnitToWegith(tUnit), available*UnitToWegith(aUnit)

	ret = Usage{
		Used:  (total - available),
		Total: total,
	}

	return
}

func GetGPULoad() (ret Usage, reterr error) {
	data, reterr := ioutil.ReadFile("/sys/devices/gpu.0/load")
	if reterr != nil {
		return
	}

	v, _ := strconv.ParseFloat(string(data), 32)
	ret.Used = float32(v)
	ret.Total = 1000
	return
}

func GetCPULoad() (ret Usage, reterr error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		reterr = err
		return
	}

	reader := bufio.NewReader(f)
	cpuLine, _, err := reader.ReadLine()
	f.Close()

	eles := strings.Split(string(cpuLine), " ")
	var vals []float32
	for i := 1; i < len(eles); i++ {
		if len(eles[i]) > 0 {
			v, _ := strconv.ParseFloat(eles[i], 32)
			vals = append(vals, float32(v))
		}

		if len(vals) >= 5 {
			break
		}
	}

	sum := float32(0.0)
	for _, v := range vals {
		sum += v
	}

	return Usage{
		Used:  sum - vals[3],
		Total: sum,
	}, nil
}

func GetCPUCores() int32 {
	return int32(runtime.NumCPU())
}

func GetGPUCores() int32 {
	return 128
}

func GetHardwareType() string {
	data, err := ioutil.ReadFile("/proc/device-tree/model")
	if err != nil {
		if data, err = ioutil.ReadFile("/var/device-tree/model"); err != nil {
			log.Printf("file does not exist, %s", "/var/device-tree/model")
			return "unknown"
		}
	}

	version := string(data)
	if strings.HasPrefix(version, "NVIDIA Jetson Nano") {
		return "jetson_nano"
	}

	if strings.HasPrefix(version, "Raspberry Pi 3") {
		return "pi3"
	}

	if strings.HasPrefix(version, "Raspberry Pi 4") {
		return "pi4"
	}

	if strings.HasPrefix(version, "Raspberry Pi Zero") {
		return "pi0"
	}

	log.Printf("Device info = %s, unknown", version)
	return "unknown"
}

func GetDeviceRole() string {
	data, err := ioutil.ReadFile("/var/device_role")
	if err != nil {
		return "unknown"
	}

	return strings.Trim(string(data), " \n")
}

func GetGateWay() string {
	data, err := ioutil.ReadFile("/var/device_gateway")
	if err != nil {
		return "unknown"
	}

	return strings.Trim(string(data), " \n")
}

func GetDeviceName() string {
	data, err := ioutil.ReadFile("/var/device_name")
	if err != nil {
		return "unknown"
	}

	return strings.Trim(string(data), " \n")
}
