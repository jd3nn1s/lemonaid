package main

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
var lastTelemetry = telemetry.Telemetry{
	Latitude:       35.488151852742455,
	Longitude:      -119.53969199955463,
	RPM:            4321,
	Speed:          90,
	CoolantTemp:    60,
	OilTemp:        104,
	BatteryVoltage: 11.3,
	FuelRemaining:  90,
}

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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// allow all connections by default
		return true
	},
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

		//		var tempTelemetry telemetry.Telemetry

		//		err = binary.Read(bytes.NewReader([]byte(msg)), binary.LittleEndian, &tempTelemetry)

		telemetryMsg, err := telemetry.Decode(msg)

		if err != nil {
			log.Println("Cannot read bytes from nerdobd2:", err)
			continue
		}

		tempTelemetry, ok := telemetryMsg.(telemetry.Telemetry)

		var b []byte

		if ok {
			tempTelemetry.Speed = float32(int(tempTelemetry.Speed*0.6213712*10)) / 10
			lock.Lock()
			lastTelemetry = tempTelemetry
			lock.Unlock()

			b, err = tempTelemetry.JSONEncode()

			if err != nil {
				log.Println("Cannot JSON encode telemetry:", err)
				continue
			}
		} else {
			tempTiming, ok := telemetryMsg.(telemetry.Timing)
			if ok {
				b, err = tempTiming.JSONEncode()

				if err != nil {
					log.Println("Cannot JSON encode timing:", err)
					continue
				}
			}
		}
		select {
		case sendChannel <- b:
		default:
		}

		select {
		case videoSyncedSendChannel <- SyncedMessage{time.Now(), b}:
		default:
			// don't block if we can't send
		}

	}
}

// websocket clients that will be sent telemetry as it comes in
func WebSocketOutgoingHandler(w http.ResponseWriter, req *http.Request, c <-chan []byte) {
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
		var msg []byte
		if first {
			// send last telemetry frame when a new client connects
			lock.RLock()
			msg, err = lastTelemetry.JSONEncode()
			lock.RUnlock()

			first = false

			if err != nil {
				continue
			}
		} else {
			msg = <-c
		}
		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Outgoing websocket closed")
			return
		}
	}
}

// websocket clients that will be sent telemetry delayed by the amount that the video is buffered by
func WebSocketVideoSyncOutgoingHandler(w http.ResponseWriter, req *http.Request, c <-chan []byte) {
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

	for {
		msg := <-c
		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("Outgoing websocket closed")
			return
		}
	}
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

	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
