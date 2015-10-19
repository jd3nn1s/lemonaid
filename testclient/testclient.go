package main

import (
	"github.com/gorilla/websocket"
	"github.com/jd3nn1s/lemonaid/telemetry"
	"log"
	"net/url"
	"time"
)

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:80", Path: "/ws/telemetry_in"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	go func() {
		// we never expect to read anything but just in case...
		defer c.Close()
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
		}
	}()

	t := telemetry.Telemetry{
		Latitude:  35.488151852742455,
		Longitude: -119.53969199955463,

		RPM:   3300,
		Speed: 20,

		AirIntakeTemp: 60.2,
		CoolantTemp:   60,
		OilTemp:       104,

		BatteryVoltage: 11.3,
		FuelRemaining:  92,
		GasPedalAngle:  80,
	}

	timing := telemetry.Timing{
		TeamAheadName:     "Damn Cheaters",
		LapCount:          23,
		TeamAheadLapCount: 26,
	}

	speedGoingUp := true
	count := 1

	for {
		time.Sleep(100 * time.Millisecond)

		if speedGoingUp == true {
			if t.Speed == 150 {
				speedGoingUp = false
			}
			t.Speed = t.Speed + 2
			t.RPM = t.RPM + 50
		} else {
			if t.Speed == 40 {
				speedGoingUp = true
			}
			t.Speed = t.Speed - 2
			t.RPM = t.RPM - 50

		}

		bytes, err := t.Encode()
		if err != nil {
			log.Println("Unable to encode telemetry", err)
			break
		}
		err = c.WriteMessage(websocket.BinaryMessage, bytes)
		if err != nil {
			log.Println("Unable to send bytes to server", err)
			break
		}
		log.Println("Sent", t)

		if count == 10 {
			count = 0

			bytes, err = timing.Encode()
			if err != nil {
				log.Println("Unable to encode timing", err)
				break
			}
			err = c.WriteMessage(websocket.BinaryMessage, bytes)
			if err != nil {
				log.Println("Unable to send bytes to server", err)
				break
			}
			log.Println("Sent Timing:", timing)
		}
		count = count + 1
	}
}
