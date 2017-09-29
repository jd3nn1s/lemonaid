package telemetry

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"
	"unsafe"
)

// same definition and order as common.h in jd3nn1s/nerdobd2
type Header struct {
	Type uint8
}

const (
	TypeTelemetry = 1
	TypeTiming    = 2
)

type Telemetry struct {
	RPM         float32
	OilPressure float32
	Speed       float32

	FuelRemaining float32
	FuelLevel     uint8

	OilTemp        float32
	CoolantTemp    float32
	AirIntakeTemp  float32
	BatteryVoltage float32

	Latitude      float64
	Longitude     float64
	Altitude      float32
	Track         float32
	GPSSpeed      float32
	GasPedalAngle uint8
}

type TelemetryWithStatus struct {
	*Telemetry
	WarningFields []string `json:",omitempty"`
	ErrorFields   []string `json:",omitempty"`
}

type Timing struct {
	BestLap       JSONDuration
	BestLapDriver string
	LastLap       JSONDuration
	LapCount      int
	DriverName    string

	TeamAheadName     string
	TeamAheadLapCount int
	TeamAheadLastLap  JSONDuration
	TeamAheadSplit    JSONDuration
}

func (t Telemetry) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, Header{TypeTelemetry})
	if err != nil {
		return buf.Bytes(), err
	}
	err = binary.Write(buf, binary.LittleEndian, t)
	return buf.Bytes(), err
}

func (t Timing) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, Header{TypeTiming})
	if err != nil {
		return buf.Bytes(), err
	}
	enc := gob.NewEncoder(buf)
	err = enc.Encode(t)
	return buf.Bytes(), err
}

func Decode(data []byte) (interface{}, error) {
	var header Header
	reader := bytes.NewReader([]byte(data))
	err := binary.Read(reader, binary.LittleEndian, &header)
	if err != nil {
		return nil, err
	}

	if header.Type == TypeTelemetry {
		var tempTelemetry Telemetry
		err := binary.Read(reader, binary.LittleEndian, &tempTelemetry)
		return tempTelemetry, err
	} else if header.Type == TypeTiming {
		var tempTiming Timing
		dec := gob.NewDecoder(reader)
		err := dec.Decode(&tempTiming)
		return tempTiming, err
	} else {
		return nil, fmt.Errorf("Unknown message type (%d)", header.Type)
	}
}

// Timing telemetry

type JSONDuration time.Duration

func (d JSONDuration) MarshalJSON() ([]byte, error) {
	minutes := int(time.Duration(d).Minutes())
	seconds := (time.Duration(d) - (time.Duration(minutes) * time.Minute)).Seconds()
	stamp := fmt.Sprintf("\"%02d:%05.2f\"", minutes, seconds)
	return []byte(stamp), nil
}

func (t Timing) JSONEncode() ([]byte, error) {
	data := struct {
		Type string
		Data Timing
	}{
		"timing",
		t,
	}

	return json.Marshal(data)
}

func (t TelemetryWithStatus) JSONEncode() ([]byte, error) {
	data := struct {
		Type string
		Data TelemetryWithStatus
	}{
		"telemetry",
		t,
	}
	return json.Marshal(data)
}

func MaxTelemetrySize() int {
	var hdr Header
	var t Telemetry
	return int(unsafe.Sizeof(hdr) + unsafe.Sizeof(t))
}