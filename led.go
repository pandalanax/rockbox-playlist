package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ledBasePath = "/sys/class/leds"

const (
	stateLEDName = "state-led"
	sosUnit      = 200 * time.Millisecond
)

func stateLEDPath(parts ...string) string {
	segments := append([]string{ledBasePath, stateLEDName}, parts...)
	return filepath.Join(segments...)
}

func writeLEDFile(name, value string) error {
	if err := os.WriteFile(stateLEDPath(name), []byte(value), 0644); err != nil {
		return fmt.Errorf("could not write %s: %w", name, err)
	}
	return nil
}

func SetLEDTrigger(trigger string) error {
	return writeLEDFile("trigger", trigger)
}

func SetLEDBrightness(on bool) error {
	value := "0"
	if on {
		value = "1"
	}
	return writeLEDFile("brightness", value)
}

func SetLEDOn() error {
	return SetLEDTrigger("default-on")
}

func SetLEDOff() error {
	if err := SetLEDTrigger("none"); err != nil {
		return err
	}
	return SetLEDBrightness(false)
}

func SetLEDHeartbeat() error {
	return SetLEDTrigger("heartbeat")
}

func SetLEDBlink(delay time.Duration) error {
	if err := SetLEDTrigger("timer"); err != nil {
		return err
	}
	ms := fmt.Sprintf("%d", delay.Milliseconds())
	if err := writeLEDFile("delay_on", ms); err != nil {
		return err
	}
	return writeLEDFile("delay_off", ms)
}

func BlinkSOSForever() error {
	if err := SetLEDTrigger("none"); err != nil {
		return err
	}
	for {
		signalSOS()
		time.Sleep(7 * sosUnit)
	}
}

func signalSOS() {
	for i := 0; i < 3; i++ {
		blinkUnit(1)
	}
	time.Sleep(2 * sosUnit)
	for i := 0; i < 3; i++ {
		blinkUnit(3)
	}
	time.Sleep(2 * sosUnit)
	for i := 0; i < 3; i++ {
		blinkUnit(1)
	}
}

func blinkUnit(units int) {
	_ = SetLEDBrightness(true)
	time.Sleep(time.Duration(units) * sosUnit)
	_ = SetLEDBrightness(false)
	time.Sleep(sosUnit)
}
