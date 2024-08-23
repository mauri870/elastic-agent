package runtimetracer

import (
	"errors"
	"io"

	"golang.org/x/exp/trace"
)

var flightRecorder *trace.FlightRecorder

// Start starts the flight recorder. It returns an error if the flight recorder is already started.
func Start() error {
	if flightRecorder != nil {
		return errors.New("flight recorder already started")
	}

	flightRecorder = trace.NewFlightRecorder()
	flightRecorder.SetSize(10 << 20) // 10MB
	return flightRecorder.Start()
}

// Stop stops the flight recorder. It returns an error if the flight recorder is not started.
func Stop() error {
	if flightRecorder == nil {
		return errors.New("flight recorder not started")
	}

	err := flightRecorder.Stop()
	if err != nil {
		return err
	}

	flightRecorder = nil
	return nil
}

// WriteTo writes the flight recorder data to the given writer.
func WriteTo(w io.Writer) (total int, err error) {
	return flightRecorder.WriteTo(w)
}
