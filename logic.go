package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type logModel struct {
	Log       string   `json:"log"`
	Timestamp string   `json:"time"`
	Tags      []string `json:"tags"`
}

type concurrentSlice struct {
	sync.RWMutex
	items []logModel
}

var cache concurrentSlice = concurrentSlice{}
var localcache concurrentSlice = concurrentSlice{}
var runTime string
var logPath = "."
var useTLS = true

func SetUseTLS(use bool) {
	useTLS = use
}

func Log(args ...interface{}) {
	go func(args ...interface{}) {
		prnt(ColorWhite, args...)
		str := ""
		for _, element := range args {
			str += fmt.Sprint(element) + " "
		}
		go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
		cache.RWMutex.Lock()
		defer cache.RWMutex.Unlock()

		cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")})
	}(args)
}

func LogColor(silent bool, color string, args ...interface{}) {
	go func(args ...interface{}) {
		if !silent {
			prnt(color, args...)
		}
		str := ""
		for _, element := range args {
			str += fmt.Sprint(element) + " "
		}
		go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
		cache.RWMutex.Lock()
		defer cache.RWMutex.Unlock()

		cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")})
	}(args)
}

func LogPanicKill(exitCode int, args ...interface{}) {
	prnt(ColorRed, args...)
	str := "PANIC: "
	for _, element := range args {
		str += fmt.Sprint(element) + " "
	}
	go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
	cache.RWMutex.Lock()
	defer cache.RWMutex.Unlock()
	cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")})
	os.Exit(exitCode)
}

func LogTagged(silent bool, color string, tags []string, args ...interface{}) {
	go func(tags []string, args ...interface{}) {
		if !silent {
			prnt(color, args...)
		}
		str := ""
		for _, element := range args {
			str += fmt.Sprint(element) + " "
		}
		go localLog(str, time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
		cache.RWMutex.Lock()
		defer cache.RWMutex.Unlock()
		cache.items = append(cache.items, logModel{Log: str, Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"), Tags: tags})
	}(tags, args)
}

func localLog(msg string, time string) {
	localcache.RWMutex.Lock()
	defer localcache.RWMutex.Unlock()

	if runTime == "" {
		localcache.items = append(localcache.items, logModel{Log: msg, Timestamp: time})
		return
	}
	if !exists(logPath + "/logs") {
		os.Mkdir(logPath+"/logs", 0755)
	}
	f, err := os.OpenFile(logPath+"/logs/"+runTime+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	for _, elements := range localcache.items {
		if _, err := f.Write([]byte(elements.Timestamp + ":" + elements.Log + "\n")); err != nil {
			log.Fatal(err)
		}
	}
	localcache.items = []logModel{}
	if _, err := f.Write([]byte(time + ":" + msg + "\n")); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

const defaultFileContent = "{\n\t\"remote_logs\": true, \n\t\"project_key\": \"<GET THIS FROM SERVER ADMINISTRATOR>\", \n\t\"host\": \"127.0.0.1\", \n\t\"port\": 12555, \n\t\"client\": \"loguser2\", \n\t\"client_key\": \"b930ffce-d388-43fc-aa1a-13962a7d6bc9\" \n}"

type PourConfig struct {
	RemoteLogs bool   `json:"remote_logs"`
	ProjectKey string `json:"project_key"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Client     string `json:"client"`
	ClientKey  string `json:"client_key"`
}

var config PourConfig

// Setups up the logging connection, host and port point to the logging server, key and project build the auth required to communicate with it.
// The doRemote flag decides whether logs are sent to the remote server or are simply locally logged. isDocker is needed to distinguish between writable file paths.
func Setup(isDocker bool) {
	if isDocker {
		logPath = "./data"
		if !exists("./data") {
			os.Mkdir("./data", 0755)
		}
	}

	if !exists(logPath + "/config_pour.json") {
		file, err := os.Create(logPath + "/config_pour.json")
		if err != nil {
			Log(false, ColorRed, "Error auto-creating pour config:", err)
			return
		}
		_, err = file.WriteString(defaultFileContent)
		if err != nil {
			Log(false, ColorRed, "Error auto-filling pour config:", err)
			return
		}

		LogPanicKill(-1, "Pour-Config ("+logPath+"/config_pour.json) was created, please fill out and restart the server")
		return
	}

	contents, err := os.ReadFile(logPath + "/config_pour.json")
	if err != nil {
		LogPanicKill(-1, "Couldn't read pour config")
		return
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		LogPanicKill(-1, "Couldn't read pour config")
		return
	}

	if config.Host == "" || config.Port <= 0 || config.ProjectKey == "" || config.Client == "" || config.ClientKey == "" {
		Log(false, ColorPurple, "LogServer values invalid, falling back to local")
	}

	Log(false, ColorPurple, "Log-Server configured at", config.Host+":"+fmt.Sprint(config.Port))

	runTime = time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	runTime = strings.ReplaceAll(runTime, ":", "_")

	Log(false, ColorGreen, "Pour up and running..")
	go logLoop(config.Host, uint(config.Port), config.ProjectKey, config.RemoteLogs, config.Client, config.ClientKey)
}

func logLoop(host string, port uint, key string, doRemote bool, client string, clientKey string) {
	for {
		time.Sleep(time.Second * 5)
		if doRemote && len(cache.items) > 0 {
			remoteLog(cache.items, host, port, key, client, clientKey)

		}
	}
}

var client = http.Client{Transport: &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // <--- Problem
}}

func remoteLog(logs []logModel, host string, port uint, key string, logClient string, clientKey string) error {
	cache.RWMutex.Lock()
	defer cache.RWMutex.Unlock()
	b, err := json.Marshal(&logs)
	if err != nil {
		Log(false, ColorRed, "Error marshalling logs", err)
		return err
	}
	httpPrefix := "http://"
	if useTLS {
		httpPrefix = "https://"
	}
	req, err := http.NewRequest("POST", httpPrefix+host+":"+fmt.Sprint(port)+"/logs", strings.NewReader(string(b)))
	if err != nil {
		//Handle Error
		Log(false, ColorRed, "Error marshalling logs", err)
		return err
	}

	req.Header.Add("X-CLIENT", logClient)
	req.Header.Add("Authorization", clientKey)
	req.Header.Add("X-KEY", key)

	res, err := client.Do(req)
	if err != nil {
		Log(false, ColorRed, "Error transmitting logs", err)
		return err
	}
	if res.StatusCode == http.StatusAccepted {
		cache.items = []logModel{}
	} else {
		defer res.Body.Close()
		read, err := io.ReadAll(res.Body)
		if err == nil {
			Log(false, ColorRed, "Error logging", string(read))
		} else {
			Log(false, ColorRed, "Error logging", res.StatusCode)
		}

	}
	return nil
}

func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

const ColorReset = "\033[0m"
const ColorGreen = "\033[32m"
const ColorYellow = "\033[33m"
const ColorBlue = "\033[34m"
const ColorPurple = "\033[35m"
const ColorCyan = "\033[36m"
const ColorWhite = "\033[37m"
const ColorRed = "\033[31m"

func prnt(color string, args ...interface{}) {
	fmt.Print("[SERVER] " + time.Now().Format(time.RFC822))
	text := ""
	for _, element := range args {
		text += fmt.Sprint(element)
		text += " "
	}
	fmt.Println(string(color), text)
	fmt.Print(ColorWhite)
}
