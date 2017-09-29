package main

import (
	"github.com/gorilla/mux"
	"github.com/jd3nn1s/lemonaid/telemetry"
	"log"
	"net/http"
	"sync"
	"time"
)

// used to control access to lastTelemetry
var lock sync.RWMutex

// used by the websocket handler that is receiving the uplinked telemetry to
// broadcast to clients
var sendChannel chan []byte

var videoSyncedSendChannel chan SyncedMessage

var videoDelay time.Duration = time.Second * 63

// Video synchronized message
type SyncedMessage struct {
	QueuedTime time.Time
	Message    []byte
}

// contains the last received telemetry values so that it
// can be sent to newly connecting clients between receives
// of telemetry and any HTTP requests
var lastTelemetry = telemetry.TelemetryWithStatus{
	Telemetry: &telemetry.Telemetry{
		Latitude:       35.488151852742455,
		Longitude:      -119.53969199955463,
		RPM:            4321,
		Speed:          90,
		CoolantTemp:    60,
		OilTemp:        104,
		BatteryVoltage: 11.3,
		FuelRemaining:  90,
	},
	WarningFields: []string{""},
	ErrorFields:   []string{""},
}

var lastTimingTelemetry = telemetry.Timing{}

// HTTP request handler for telemetry
func TelemetryServer(w http.ResponseWriter, req *http.Request) {
	lock.RLock()
	b, err := lastTelemetry.JSONEncode()
	lock.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func main() {
	// Realtime broadcast channels
	sendChannel = make(chan []byte, 3)
	addChannel := make(chan chan []byte)
	delChannel := make(chan chan []byte)

	// broadcaster go function: receives telemetry and sends to all
	// registered client channels. Client channels are added and deleted
	// via additional channels
	go func() {
		outgoingWebChannels := make([]chan []byte, 0, 100)
		for {
			select {
			case c := <-addChannel:
				log.Println("Adding client")
				outgoingWebChannels = append(outgoingWebChannels, c)
			case c := <-delChannel:
				log.Println("Removing client")
				b := outgoingWebChannels[:0]
				for _, x := range outgoingWebChannels {
					if x != c {
						b = append(b, x)
					}
				}
				outgoingWebChannels = b
			case t := <-sendChannel:
				for c := range outgoingWebChannels {
					select {
					case outgoingWebChannels[c] <- t:
					default:
					}
				}
			}
		}
	}()

	udpServer, err := NewUDPIncoming()
	if err != nil {
		log.Println("unable to create UDP server", err)
		return
	}
	udpServer.Start()
	defer udpServer.Close()

	// assume we can't back-up more than 3600 telemetry messages
	videoSyncedSendChannel = make(chan SyncedMessage, 3600)
	videoSyncedAddChannel := make(chan chan []byte)
	videoSyncedDelChannel := make(chan chan []byte)

	// broadcaster go routine that delays telemetry messages to clients
	// so that telemetry can be synchronized with video.
	go func() {
		outgoingWebChannels := make([]chan []byte, 0, 100)
		var syncedTimerChannel <-chan time.Time
		var lastSyncedMessage SyncedMessage
		for {
			select {
			case c := <-videoSyncedAddChannel:
				log.Println("Adding videosynced client")
				outgoingWebChannels = append(outgoingWebChannels, c)
			case c := <-videoSyncedDelChannel:
				log.Println("Removing videosynced client")
				b := outgoingWebChannels[:0]
				for _, x := range outgoingWebChannels {
					if x != c {
						b = append(b, x)
					}
				}
				outgoingWebChannels = b
			case lastSyncedMessage := <-videoSyncedSendChannel:
				sendAt := lastSyncedMessage.QueuedTime.Add(videoDelay)
				syncedTimerChannel = time.After(sendAt.Sub(time.Now()))
			case <-syncedTimerChannel:
				for c := range outgoingWebChannels {
					select {
					case outgoingWebChannels[c] <- lastSyncedMessage.Message:
					default:
					}
				}
			}
		}
	}()

	// connect to timing server
	timingTelemetry := make(chan telemetry.Timing)
	go func() {
		for {
			tmpTelemetry := <-timingTelemetry
			lock.Lock()
			lastTimingTelemetry = tmpTelemetry
			lock.Unlock()
			msg, err := tmpTelemetry.JSONEncode()
			if err == nil {
				log.Println("SENDING TO CHANNEL")
				sendChannel <- msg
			} else {
				log.Println("ERROR", err)
			}
		}
	}()
	//go startLapTimes(timingTelemetry)

	r := mux.NewRouter()
	r.HandleFunc("/telemetry", TelemetryServer).Methods("GET")
	r.HandleFunc("/ws/telemetry_in", WebSocketIncomingHandler)
	r.HandleFunc("/ws/telemetry_out", func(w http.ResponseWriter, req *http.Request) {
		channel := make(chan []byte, 3)
		addChannel <- channel
		WebSocketOutgoingHandler(w, req, channel)
		delChannel <- channel
	})
	r.HandleFunc("/ws/telemetry_out/videosync", func(w http.ResponseWriter, req *http.Request) {
		channel := make(chan []byte, 3)
		videoSyncedAddChannel <- channel
		WebSocketVideoSyncOutgoingHandler(w, req, channel)
		videoSyncedDelChannel <- channel
	})
	RegisterSiteHandlers(r)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/", r)


	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
