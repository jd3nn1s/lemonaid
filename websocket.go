package main

import (
	"time"
	"net/http"
	"github.com/gorilla/websocket"
	"log"
	"github.com/jd3nn1s/lemonaid/telemetry"
	"github.com/pkg/errors"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// allow all connections by default
		return true
	},
}

func ProcessMsg(msg []byte) error {
	telemetryMsg, err := telemetry.Decode(msg)

	if err != nil {
		return errors.Wrap(err, "cannot read bytes from nerdobd2:")

	}

	tempTelemetry, ok := telemetryMsg.(telemetry.Telemetry)

	var b []byte

	if ok {
		tempTelemetry.Speed = float32(int(tempTelemetry.Speed*0.6213712*10)) / 10
		// add error/warning fields
		var warnings []string
		var errs []string

		if tempTelemetry.FuelLevel < 5 {
			errs = append(errs, []string{"FuelRemaining", "FuelLevel"}...)
		} else if tempTelemetry.FuelLevel < 20 {
			warnings = append(warnings, []string{"FuelRemaining", "FuelLevel"}...)
		}

		if tempTelemetry.CoolantTemp > 110 {
			errs = append(errs, "CoolantTemp")
		} else if tempTelemetry.CoolantTemp > 105 {
			warnings = append(warnings, "CoolantTemp")
		}

		if tempTelemetry.BatteryVoltage < 11 {
			errs = append(errs, "BatteryVoltage")
		} else if tempTelemetry.BatteryVoltage < 12 {
			warnings = append(warnings, "BatteryVoltage")
		}

		if tempTelemetry.OilTemp > 260 {
			errs = append(errs, "OilTemp")
		} else if tempTelemetry.OilTemp > 250 {
			warnings = append(warnings, "OilTemp")
		}

		telemetryWithStatus := telemetry.TelemetryWithStatus{
			&tempTelemetry,
			warnings,
			errs,
		}

		lock.Lock()
		lastTelemetry = telemetryWithStatus
		lock.Unlock()

		b, err = telemetryWithStatus.JSONEncode()

		if err != nil {
			return errors.Wrap(err, "cannot json encode telemetry")
		}
	} else {
		tempTiming, ok := telemetryMsg.(telemetry.Timing)
		if ok {
			b, err = tempTiming.JSONEncode()

			if err != nil {
				return errors.Wrap(err, "cannot JSON encode timing")
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
	return nil
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

		if err = ProcessMsg(msg); err != nil {
			log.Println("unable to process websocket msg:", err)
			continue
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
			msgTiming, errTiming := lastTimingTelemetry.JSONEncode()
			lock.RUnlock()
			if errTiming != nil {
				conn.WriteMessage(websocket.TextMessage, msgTiming)
			}

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
