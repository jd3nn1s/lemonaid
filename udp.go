package main

import (
	"net"
	"github.com/pkg/errors"
	"sync"
	"log"
	"github.com/jd3nn1s/lemonaid/telemetry"
)

type UDPIncoming struct {
	serverCon *net.UDPConn
	wg sync.WaitGroup
}

const udpPort = "2020"

func NewUDPIncoming() (*UDPIncoming, error) {
	return &UDPIncoming{}, nil
}

func (udp *UDPIncoming) Start() error {
	serverAddr, err := net.ResolveUDPAddr("udp", ":" + udpPort)
	if err != nil {
		return errors.Wrapf(err, "unable to start udp incoming")
	}

	udp.serverCon, err = net.ListenUDP("udp", serverAddr)
	if err != nil {
		return errors.Wrapf(err, "unable to start listening on udp port")
	}

	if err = udp.serverCon.SetReadBuffer(telemetry.MaxTelemetrySize() * 2); err != nil {
		return errors.Wrapf(err, "unable to set udp read buffer size")
	}

	udp.wg.Add(1)
	go func() {
		defer udp.wg.Done()

		buf := make([]byte, telemetry.MaxTelemetrySize() * 2)
		for {
			_, _, err := udp.serverCon.ReadFromUDP(buf)
			if err != nil {
				log.Println("error when reading udp socket", err)
				return
			}
			if err := ProcessMsg(buf); err != nil {
				log.Println("unable to process udp message:", err)
			}
		}
	}()

	return nil
}

func (udp *UDPIncoming) Close() {
	if udp.serverCon != nil {
		udp.serverCon.Close()
		udp.wg.Wait()
		udp.serverCon = nil
	}
}