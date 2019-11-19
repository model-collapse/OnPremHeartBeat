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
var rootName = flag.String("root_name", "devices", "root node of ZK for device info")
var urlTmpl = `http://%s:18969/heartbeat?device=%s`

func CreateIfNotExistAndUpdate(name string, val []byte) {
	dtp := DevicePropertyPath(name)
	if exists, _, err := ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		ZKConn.Create(dtp, val, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	} else {
		ZKConn.Set(dtp, val, 0)
	}
}

func WriteBasicInfoViaZK() {
	dt := GetHardwareType()
	rol := GetDeviceRole()

	CreateIfNotExistAndUpdate("device_type", []byte(dt))
	CreateIfNotExistAndUpdate("device_role", []byte(rol))

	cpuc := fmt.Sprintf("%d", GetCPUCores())
	mem, _ := GetMemory()
	cpuu, _ := GetCPULoad()

	CreateIfNotExistAndUpdate("cpu_cores", []byte(cpuc))
	CreateIfNotExistAndUpdate("cpu", []byte(fmt.Sprintf("%f", cpuu.Used/cpuu.Total)))
	CreateIfNotExistAndUpdate("mem_cap", []byte(fmt.Sprintf("%d", int(mem.Total))))
	CreateIfNotExistAndUpdate("mem", []byte(fmt.Sprintf("%f", mem.Used/mem.Total)))

	if dt == "jetson_nano" {
		gpuc := fmt.Sprintf("%d", GetGPUCores())
		gpuu, _ := GetGPULoad()
		CreateIfNotExistAndUpdate("gpu_cores", []byte(gpuc))
		CreateIfNotExistAndUpdate("gpu", []byte(fmt.Sprintf("%f", gpuu.Used/gpuu.Total)))
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

	if len(*deviceName) == 0 {
		dn := GetDeviceName()
		deviceName = &dn
	}

	hInterval := time.Second * time.Duration(*interval)
	ticker := time.NewTicker(hInterval)

	var f func()
	role := GetDeviceRole()
	if role == "edge" {
		log.Printf("Start heatbeating as an edge device...")
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
	}

	for range ticker.C {
		log.Printf("Sending heatbeat...")
		f()
	}
}
