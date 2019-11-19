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
	CreateIfNotExistAndUpdateAbs("/"+*rootName, []byte("nothing"), false)
	CreateIfNotExistAndUpdateAbs(fmt.Sprintf("/%s/%s", *rootName, *deviceName), []byte("nothing"), false)
}

func DevicePropertyPath(name string) string {
	return fmt.Sprintf("/%s/%s/%s", *rootName, *deviceName, name)
}

func CreateIfNotExistAndUpdate(name string, val []byte, needUpdate bool) {
	dtp := DevicePropertyPath(name)
	if exists, _, err := ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		if _, err := ZKConn.Create(dtp, val, 0, zk.WorldACL(zk.PermAll)); err != nil {
			log.Printf("Fail to create node %s, %v", dtp, err)
		}
	} else if needUpdate {
		ZKConn.Set(dtp, val, 0)
	}
}

func CreateIfNotExistAndUpdateAbs(dtp string, val []byte, needUpdate bool) {
	if exists, _, err := ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		if _, err := ZKConn.Create(dtp, val, 0, zk.WorldACL(zk.PermAll)); err != nil {
			log.Printf("Fail to create node %s, %v", dtp, err)
		}
	} else if needUpdate {
		ZKConn.Set(dtp, val, 0)
	}
}

func WriteBasicInfoViaZK() {
	dt := GetHardwareType()
	rol := GetDeviceRole()

	CreateIfNotExistAndUpdate("device_type", []byte(dt), true)
	CreateIfNotExistAndUpdate("device_role", []byte(rol), true)

	cpuc := fmt.Sprintf("%d", GetCPUCores())
	mem, _ := GetMemory()
	cpuu, _ := GetCPULoad()

	CreateIfNotExistAndUpdate("cpu_cores", []byte(cpuc), true)
	CreateIfNotExistAndUpdate("cpu", []byte(fmt.Sprintf("%f", cpuu.Used/cpuu.Total)), true)
	CreateIfNotExistAndUpdate("mem_cap", []byte(fmt.Sprintf("%d", int(mem.Total))), true)
	CreateIfNotExistAndUpdate("mem", []byte(fmt.Sprintf("%f", mem.Used/mem.Total)), true)

	if dt == "jetson_nano" {
		gpuc := fmt.Sprintf("%d", GetGPUCores())
		gpuu, _ := GetGPULoad()
		CreateIfNotExistAndUpdate("gpu_cores", []byte(gpuc), true)
		CreateIfNotExistAndUpdate("gpu", []byte(fmt.Sprintf("%f", gpuu.Used/gpuu.Total)), true)
	}
}

func SendUsageViaZK() {
	cpuu, _ := GetCPULoad()
	gpuu, _ := GetGPULoad()
	mem, _ := GetMemory()

	ZKConn.Set(DevicePropertyPath("cpu"), []byte(fmt.Sprintf("%f", cpuu)), 0)
	ZKConn.Set(DevicePropertyPath("gpu"), []byte(fmt.Sprintf("%f", gpuu)), 0)
	ZKConn.Set(DevicePropertyPath("mem"), []byte(fmt.Sprintf("%f", mem)), 0)

	heartbeat := fmt.Sprintf("%d", time.Now().Unix())
	ZKConn.Set(DevicePropertyPath("heatbeat"), []byte(heartbeat), 0)
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
		WriteBasicInfoViaZK()
		SendUsageViaZK()
		f = func() {
			SendUsageViaZK()
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
