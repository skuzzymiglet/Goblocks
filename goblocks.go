package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

type configStruct struct {
	Separator          string
	ConfigReloadSignal int //currently unused
	Actions            []map[string]interface{}
}

var blocks []string
var channels []chan bool
var signalMap map[string]int = make(map[string]int)

func main() {
	confDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	config, err := readConfig(filepath.Join(confDir, "goblocks.json"))
	if err != nil {
		log.Fatal(err)
	}
	channels = make([]chan bool, len(config.Actions))
	//recChannel is common for goroutines contributing to status bar
	recChannel := make(chan Change)
	for i, action := range config.Actions {
		//Assign a cell for each separator/prefix/action/suffix
		if config.Separator != "" {
			blocks = append(blocks, config.Separator)
		}
		if value, ok := action["prefix"]; ok {
			blocks = append(blocks, value.(string))
		}
		blocks = append(blocks, "action")
		actionID := len(blocks) - 1
		if value, ok := action["suffix"]; ok {
			blocks = append(blocks, value.(string))
		}
		//Create an unique channel for each action
		channels[i] = make(chan bool)
		signalMap["signal "+action["updateSignal"].(string)] = i
		go RunCmd(actionID, recChannel, channels[i], action)
		timer := action["timer"].(string)
		if timer != "0" {
			go Schedule(channels[i], timer)
		}
	}
	go handleSignals(GetSIGRTchannel())
	var updates float64
	start := time.Now()
	for res := range recChannel {
		//Block until some goroutine has an update
		if res.Success {
			blocks[res.BlockID] = res.Data
		} else {
			fmt.Println(res.Data)
			blocks[res.BlockID] = "ERROR"
		}
		if err = updateStatusBar(); err != nil {
			log.Fatalf("failed to update status bar: %s\n", err)
		}
		updates++
		perSec := updates / float64(time.Now().Sub(start)) * float64(time.Second)
		// log.Printf("%f updates per second", perSec)
		if perSec >= 20 && !(updates == 100) {
			log.Printf("Too many updates (%f) exiting to avoid crash", perSec)
		}
	}
}

//Read config and map it to configStruct
func readConfig(path string) (config configStruct, err error) {
	var file *os.File
	file, err = os.Open(path)
	defer file.Close()
	if err != nil {
		return config, err
	}
	var byteValue []byte
	byteValue, err = ioutil.ReadAll(file)
	err = json.Unmarshal([]byte(byteValue), &config)
	if err != nil {
		return config, err

	}
}

//Goroutine that pings a channel according to received signal
func handleSignals(rec chan os.Signal) {
	for sig := range rec {
		if index, ok := signalMap[sig.String()]; ok {
			channels[index] <- true
		}
	}
}

//Craft status text out of blocks data
func updateBar(conn *xgb.Conn, rootWindow xproto.Window) {
	var builder strings.Builder
	for _, s := range blocks {
		builder.WriteString(s)
	}
	xproto.ChangeProperty(conn, xproto.PropModeReplace, rootWindow, xproto.AtomWmName, xproto.AtomString, 8, uint32(builder.Len()), []byte(builder.String()))
}
