package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type Monitor struct {
	DisplayNameTarget      string        `json:"displayNameTarget"`
	IntervalInMilliseconds time.Duration `json:"intervalInMilliseconds"`
	TimeoutInMilliseconds  time.Duration `json:"timeoutInMilliseconds"`
	Type                   string        `json:"type"`
	ProtocolName           string        `json:"protocolName"`
	Destination            string        `json:"destination"`
	Port                   int           `json:"port"`
}

type SimpleWebMonitorReport struct {
	StartTime    time.Time
	StatusCode   int
	IsOnline     int
	ResponseTime int64
	EndTime      time.Time
	Name         string
	Source       string
}

type ConfigFile struct {
	BackendConfig struct {
		Protocol     string `json:"protocol"`
		Host         string `json:"host"`
		Port         int    `json:"port"`
		Organisation string `json:"organisation"`
		Bucket       string `json:"bucket"`
		Username     string `json:"username"`
		Password     string `json:"password"`
	} `json:"backendConfig"`
	LocalConfig struct {
		DisplayNameSource string `json:"displayNameSource"`
	} `json:"localConfig"`
	Monitors []Monitor `json:"monitors"`
}

const ALLOWED_TYPE_simplePortMonitor = "simplePortMonitor"
const ALLOWED_TYPE_simpleWebMonitor = "simpleWebMonitor"

func main() {
	log.Println("Starting gomonitor....")
	jsonFile, err := os.Open("monitoring_config.json")
	if err != nil {
		log.Panicln(err)
	}
	log.Println("Found monitoring_config.json")
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var configFile ConfigFile
	json.Unmarshal(byteValue, &configFile)
	log.Println("Parsed monitoring_config.json")

	// initialize local settings
	var wg sync.WaitGroup
	var displaySourceName = configFile.LocalConfig.DisplayNameSource
	var backendProtocol = configFile.BackendConfig.Protocol
	var backendHost = configFile.BackendConfig.Host
	var backendPort = configFile.BackendConfig.Port
	var backendUsername = configFile.BackendConfig.Username
	var backendPassword = configFile.BackendConfig.Password
	var backendOrganisation = configFile.BackendConfig.Organisation
	var backendBucket = configFile.BackendConfig.Bucket

	log.Println("Using " + displaySourceName + " as source name")

	// connect to backend
	client := influxdb2.NewClient(backendProtocol+"://"+backendHost+":"+strconv.Itoa(backendPort), fmt.Sprintf("%s:%s", backendUsername, backendPassword))
	async := client.WriteAPI(backendOrganisation, backendBucket)

	log.Println("Setup backend done")
	log.Println("Starting " + strconv.Itoa(len(configFile.Monitors)) + " monitors")
	// starting monitoring
	for i := 0; i < len(configFile.Monitors); i++ {
		switch configFile.Monitors[i].Type {
		case ALLOWED_TYPE_simplePortMonitor:
			wg.Add(1)
			go simplePortMonitor(async, configFile.Monitors[i])
		case ALLOWED_TYPE_simpleWebMonitor:
			wg.Add(1)
			go simpleWebMonitor(async, configFile.Monitors[i])
		default:
			log.Println("Type not known or implemented: " + configFile.Monitors[i].Type)
		}
	}
	// will never end by default
	wg.Wait()
	async.Flush()
	// Close client
	client.Close()
}

// begin simplePortMonitor
func simplePortMonitor(async api.WriteAPI, monitor Monitor) {
	tick := time.Tick(monitor.IntervalInMilliseconds * time.Millisecond)
	for range tick {
		//log.Println("Tick: " + check.destinationHost)
		//go monitorHTTPHost(wg, async, check)

	}
}

// end simplePortMonitor
// begin simpleWebMonitor

func simpleWebMonitor(async api.WriteAPI, monitor Monitor) {
	tick := time.Tick(monitor.IntervalInMilliseconds * time.Millisecond)
	for range tick {
		go simpleWebMonitorCheck(async, monitor)
	}
}

func simpleWebMonitorCheck(async api.WriteAPI, monitor Monitor) {
	var simpleWebMonitorResult SimpleWebMonitorReport
	simpleWebMonitorResult.StartTime = time.Now()
	simpleWebMonitorResult.Name = monitor.DisplayNameTarget
	simpleWebMonitorResult.StatusCode = 0
	simpleWebMonitorResult.IsOnline = 0

	req, err := http.NewRequest("GET", monitor.ProtocolName+"://"+monitor.Destination+":"+strconv.Itoa(monitor.Port), nil)
	if err != nil {
		log.Fatal("Error reading request. ", err)
		return
	}
	req.Header.Set("Cache-Control", "no-cache")

	customTransport := &(*http.DefaultTransport.(*http.Transport)) // make shallow copy
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := http.Client{Timeout: time.Millisecond * monitor.TimeoutInMilliseconds, Transport: customTransport}

	resp, err := client.Do(req)
	if err == nil {
		simpleWebMonitorResult.StatusCode = resp.StatusCode
		simpleWebMonitorResult.IsOnline = 1
		defer resp.Body.Close()
	}
	simpleWebMonitorResult.ResponseTime = time.Since(simpleWebMonitorResult.StartTime).Milliseconds()
	simpleWebMonitorResult.EndTime = time.Now()

	// do reporting
	simpleWebMonitorReport(async, simpleWebMonitorResult)
}

func simpleWebMonitorReport(async api.WriteAPI, monitorResult SimpleWebMonitorReport) {
	log.Println("hztrt")
	p := influxdb2.NewPointWithMeasurement(ALLOWED_TYPE_simpleWebMonitor).
		AddTag("name", monitorResult.Name).
		AddTag("source", monitorResult.Source).
		AddField("statusCode", monitorResult.StatusCode).
		AddField("responseTime", monitorResult.ResponseTime).
		AddField("isOnline", monitorResult.IsOnline).
		SetTime(monitorResult.StartTime)
	async.WritePoint(p)
}

// end simpleWebMonitor
