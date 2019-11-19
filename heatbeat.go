package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

var interval = flag.Int("interval", 10, "seconds of interval on sending the heatbeat")
var deviceName = flag.String("device_name", "", "device name")
var rootName = flag.String("root_name", "heartbeats", "root node of ZK for device info")
var zkPath = flag.String("zk_addr", "n1.onprem.ai:2181", "path to zk")
var urlTmpl = `http://%s:18969/heartbeat?device=%s`

func RegisterDevice() {
	CreateIfNotExistAndUpdateAbs("/"+*rootName, []byte("nothing"), false, 0)
	CreateIfNotExistAndUpdateAbs(fmt.Sprintf("/%s/%s", *rootName, *deviceName), []byte("nothing"), false, 0)
}

func DevicePropertyPath(name string) string {
	return fmt.Sprintf("/%s/%s/%s", *rootName, *deviceName, name)
}

func ZKSetAndLog(path string, val []byte, ver int32) {
	if _, err := ZKConn.Set(path, val, ver); err != nil {
		log.Printf("Error in setting %s to %s, %v", path, val, err)
	}
}

func GetVersion() (int32, error) {
	_, stat, err := ZKConn.Exists(fmt.Sprintf("/%s/%$/cpu", *rootName, *deviceName))
	if err != nil {
		return 0, err
	}

	return stat.Version, nil
}

func CreateIfNotExistAndUpdate(name string, val []byte, needUpdate bool, ver int32) {
	dtp := DevicePropertyPath(name)
	if exists, _, err := ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		if _, err := ZKConn.Create(dtp, val, 0, zk.WorldACL(zk.PermAll)); err != nil {
			log.Printf("Fail to create node %s, %v", dtp, err)
		}
	} else if needUpdate {
		ZKSetAndLog(dtp, val, ver)
	}
}

func CreateIfNotExistAndUpdateAbs(dtp string, val []byte, needUpdate bool, ver int32) {
	if exists, _, err := ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		if _, err := ZKConn.Create(dtp, val, 0, zk.WorldACL(zk.PermAll)); err != nil {
			log.Printf("Fail to create node %s, %v", dtp, err)
		}
	} else if needUpdate {
		ZKSetAndLog(dtp, val, ver)
	}
}

func WriteBasicInfoViaZK() (dt string, ver int32) {
	dt = GetHardwareType()
	rol := GetDeviceRole()

	ver, err := GetVersion()
	if err != nil {
		log.Printf("Error in getting version, %s", err)
	}
	log.Printf("Setting version from %d to %d", ver)
	ver++

	CreateIfNotExistAndUpdate("device_type", []byte(dt), true, ver)
	CreateIfNotExistAndUpdate("device_role", []byte(rol), true, ver)

	cpuc := fmt.Sprintf("%d", GetCPUCores())
	mem, _ := GetMemory()
	cpuu, _ := GetCPULoad()

	CreateIfNotExistAndUpdate("cpu_cores", []byte(cpuc), true, ver)
	CreateIfNotExistAndUpdate("cpu", []byte(fmt.Sprintf("%f", cpuu.Used/cpuu.Total)), true, ver)
	CreateIfNotExistAndUpdate("mem_cap", []byte(fmt.Sprintf("%d", int(mem.Total))), true, ver)
	CreateIfNotExistAndUpdate("mem", []byte(fmt.Sprintf("%f", mem.Used/mem.Total)), true, ver)

	if dt == "jetson_nano" {
		gpuc := fmt.Sprintf("%d", GetGPUCores())
		gpuu, _ := GetGPULoad()
		CreateIfNotExistAndUpdate("gpu_cores", []byte(gpuc), true, ver)
		CreateIfNotExistAndUpdate("gpu", []byte(fmt.Sprintf("%f", gpuu.Used/gpuu.Total)), true, ver)
	}

	return dt, ver
}

func SendUsageViaZK(dt string, ver int32) {
	cpuu, _ := GetCPULoad()
	gpuu, _ := GetGPULoad()
	mem, _ := GetMemory()

	ZKSetAndLog(DevicePropertyPath("cpu"), []byte(fmt.Sprintf("%f", cpuu.Used/cpuu.Total)), ver)

	if dt == "jetson_nano" {
		ZKSetAndLog(DevicePropertyPath("gpu"), []byte(fmt.Sprintf("%f", gpuu.Used/cpuu.Total)), ver)
	}

	ZKSetAndLog(DevicePropertyPath("mem"), []byte(fmt.Sprintf("%f", mem.Used/mem.Total)), ver)

	heartbeat := fmt.Sprintf("%d", time.Now().Unix())
	ZKSetAndLog(DevicePropertyPath("heatbeat"), []byte(heartbeat), ver)
}

func SendUsageViaAPI() {
	cpuu, _ := GetCPULoad()
	mem, _ := GetMemory()

	body := struct {
		DeviceTypeStr string `json:"device_type"`
		DeviceRoleStr string `json:"device_role"`

		CPU float32 `json:"cpu"`
		Mem float32 `json:"mem"`

		//device setting
		CPUCores  int `json:"cpu_cores"`
		MemoryCap int `json:"mem_cap"`

		// heart beat
		LastHeartBeat int64 `json:"heatbeat"`
	}{
		DeviceTypeStr: GetHardwareType(),
		DeviceRoleStr: GetDeviceRole(),
		CPU:           cpuu.Used / cpuu.Total,
		Mem:           mem.Used / mem.Total,
		CPUCores:      int(GetCPUCores()),
		MemoryCap:     int(mem.Total),
		LastHeartBeat: time.Now().Unix(),
	}

	bodyStr, _ := json.Marshal(body)
	url := fmt.Sprintf(urlTmpl, GetGateWay(), GetDeviceName())

	if resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyStr)); err != nil {
		log.Printf("Fail to send heatbeat via API, %v", err)
	} else if resp.StatusCode != 200 {
		log.Printf("Serverside err [%d] after posting heatbeat", resp.StatusCode)
	} else {
		log.Printf("Heatbeat successfully sent!")
	}
}

func main() {
	flag.Parse()
	InitializeZK([]string{*zkPath})

	if len(*deviceName) == 0 {
		dn := GetDeviceName()
		deviceName = &dn
	}

	hInterval := time.Second * time.Duration(*interval)
	ticker := time.NewTicker(hInterval)

	var f func()
	role := GetDeviceRole()
	log.Printf("role = %s", role)
	if role == "edge" {
		log.Printf("Start heatbeating as an edge device...")
		RegisterDevice()
		dt, ver := WriteBasicInfoViaZK()
		SendUsageViaZK(dt, ver)
		f = func() {
			ver++
			SendUsageViaZK(dt, ver)
		}
	} else if role == "sensor" {
		log.Printf("Start heatbeating as a sensor...")
		SendUsageViaAPI()
		f = func() {
			SendUsageViaAPI()
		}
	} else {
		log.Fatalf("Invalid role: %s", role)
	}

	for range ticker.C {
		log.Printf("Sending heatbeat...")
		f()
	}
}
