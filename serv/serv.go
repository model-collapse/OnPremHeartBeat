package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	rt "github.com/qiangxue/fasthttp-routing"
	"github.com/samuel/go-zookeeper/zk"
	fh "github.com/valyala/fasthttp"
)

var rootName = flag.String("root_name", "heartbeats", "root node of ZK for device info")
var zkPath = flag.String("zk_addr", "n1.onprem.ai:2181", "path to zk")

type HeartBeatRequest struct {
	DeviceTypeStr string `json:"device_type"`
	DeviceRoleStr string `json:"device_role"`

	CPU float32 `json:"cpu"`
	Mem float32 `json:"mem"`

	//device setting
	CPUCores  int `json:"cpu_cores"`
	MemoryCap int `json:"mem_cap"`

	// heart beat
	LastHeartBeat int64 `json:"heatbeat"`
}

func RegisterDevice(deviceName string) bool {
	CreateIfNotExistAndUpdateAbs("/"+*rootName, []byte("nothing"), false, 0)
	return CreateIfNotExistAndUpdateAbs(fmt.Sprintf("/%s/%s", *rootName, deviceName), []byte("nothing"), false, 0)
}

func DevicePropertyPath(device, name string) string {
	return fmt.Sprintf("/%s/%s/%s", *rootName, device, name)
}

func ZKSetAndLog(path string, val []byte, ver int32) {
	if _, err := ZKConn.Set(path, val, ver); err != nil {
		log.Printf("Error in setting %s to %s, %v", path, val, err)
	}
}

func CreateIfNotExistAndUpdate(device, name string, val []byte, needUpdate bool, ver int32) (exists bool) {
	dtp := DevicePropertyPath(device, name)
	var err error
	if exists, _, err = ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		if _, err := ZKConn.Create(dtp, val, 0, zk.WorldACL(zk.PermAll)); err != nil {
			log.Printf("Fail to create node %s, %v", dtp, err)
		}
	} else if needUpdate {
		ZKSetAndLog(dtp, val, ver)
	}

	return
}

func CreateIfNotExistAndUpdateAbs(dtp string, val []byte, needUpdate bool, ver int32) (exists bool) {
	var err error
	if exists, _, err = ZKConn.Exists(dtp); err != nil {
		log.Printf("Error checking node %s, %v", dtp, err)
	} else if !exists {
		if _, err := ZKConn.Create(dtp, val, 0, zk.WorldACL(zk.PermAll)); err != nil {
			log.Printf("Fail to create node %s, %v", dtp, err)
		}
	} else if needUpdate {
		ZKSetAndLog(dtp, val, ver)
	}

	return
}

func main() {
	InitializeZK([]string{*zkPath})

	router := rt.New()
	handler := func(ctx rt.Context) {
		deviceName := string(ctx.URI().QueryArgs().Peek("device_name"))
		var req HeartBeatRequest
		json.Unmarshal(ctx.PostBody(), &req)

		//register
		registered := RegisterDevice(deviceName)

		//write
		if !registered {
			CreateIfNotExistAndUpdate(deviceName, "device_role", []byte(req.DeviceRoleStr), true, -1)
			CreateIfNotExistAndUpdate(deviceName, "device_type", []byte(req.DeviceTypeStr), true, -1)

			CreateIfNotExistAndUpdate(deviceName, "cpu_cores", []byte(fmt.Sprintf("%d", req.CPUCores)), true, -1)
			CreateIfNotExistAndUpdate(deviceName, "mem_cap", []byte(fmt.Sprintf("%d", req.MemoryCap)), true, -1)

			CreateIfNotExistAndUpdate(deviceName, "cpu", []byte(fmt.Sprintf("%f", req.CPU)), true, -1)
			CreateIfNotExistAndUpdate(deviceName, "mem", []byte(fmt.Sprintf("%f", req.Mem)), true, -1)

			CreateIfNotExistAndUpdate(deviceName, "heartbeat", fmt.Sprintf("%d", req.LastHeartBeat)), true, -1)
		} else {
			ZKSetAndLog(DevicePropertyPath(deviceName, "cpu"), []byte(fmt.Sprintf("%f", req.CPU)), -1)
			ZKSetAndLog(DevicePropertyPath(deviceName, "mem"), []byte(fmt.Sprintf("%f", req.Mem)), -1)
			ZKSetAndLog(DevicePropertyPath(deviceName, "heartbeat"), []byte(fmt.Sprintf("%d", req.LastHeartBeat)), -1)
		}
	}
	router.Post("/heartbeat", handler)

	fh.ListenAndServe(router.HandleRequest)
}
