package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

// used to control access to lastTelemetry
var lock sync.RWMutex

// used by the websocket handler that is receiving the uplinked telemetry to
// broadcast to clients
var sendChannel chan Telemetry

// same definition and order as common.h in jd3nn1s/nerdobd2
type Telemetry struct {
	RPM           float32
	InjectionTime float32
	OilPressure   float32
	Speed         float32

	ConsumptionPerHr    float32
	ConsumptionPer100km float32

	DurationConsumption float32
	DurationSpeed       float32

	CoolantTemp   float32
	TempAirIntake float32
	Voltage       float32
}

// contains the last received telemetry values so that it
// can be sent to newly connecting clients between receives
// of telemetry and any HTTP requests
var lastTelemetry Telemetry

// HTTP request handler for telemetry
func TelemetryServer(w http.ResponseWriter, req *http.Request) {
	lock.RLock()
	b, err := json.Marshal(lastTelemetry)
	lock.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// telemetry uplink handler. Decodes binary websocket message and forwards
// to broadcaster
func WebSocketIncomingHandler(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for {
		t, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if t != websocket.BinaryMessage {
			continue
		}

		var tempTelemetry Telemetry
		err = binary.Read(bytes.NewReader([]byte(msg)), binary.LittleEndian, &tempTelemetry)
		lock.Lock()
		lastTelemetry = tempTelemetry
		lock.Unlock()

		select {
		case sendChannel <- tempTelemetry:
		default:
		}
	}
}

// websocket clients that will be sent telemetry as it comes in
func WebSocketOutgoingHandler(w http.ResponseWriter, req *http.Request, c <-chan Telemetry) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// read is required for proper websocket operation
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				conn.Close()
				break
			}
		}
	}()

	first := true
	for {
		var telemetry Telemetry
		if first {
			// send last telemetry frame when a new client connects
			lock.RLock()
			telemetry = lastTelemetry
			lock.RUnlock()
			first = false
		} else {
			telemetry = <-c
		}
		b, err := json.Marshal(telemetry)
		if err != nil {
			continue
		}
		log.Println("sending to client")
		err = conn.WriteMessage(websocket.TextMessage, b)
		if err != nil {
			log.Println("Outgoing websocket closed")
			return
		}
	}
}

func main() {
	sendChannel = make(chan Telemetry, 3)
	addChannel := make(chan chan Telemetry)
	delChannel := make(chan chan Telemetry)

	// broadcaster go function: receives telemetry and sends to all
	// registered client channels. Clietn channels are added and deleted
	// via additional channels
	go func() {
		outgoingWebChannels := make([]chan Telemetry, 0, 100)
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

	r := mux.NewRouter()
	r.HandleFunc("/telemetry", TelemetryServer).Methods("GET")
	r.HandleFunc("/ws/telemetry_in", WebSocketIncomingHandler)
	r.HandleFunc("/ws/telemetry_out", func(w http.ResponseWriter, req *http.Request) {
		channel := make(chan Telemetry, 3)
		addChannel <- channel
		WebSocketOutgoingHandler(w, req, channel)
		delChannel <- channel
	})
	RegisterSiteHandlers(r)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/", r)

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
