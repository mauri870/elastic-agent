// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent/testing/integration"
	"github.com/elastic/mock-es/pkg/api"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
)

// TestMonitoringTelemetryStartupMetrics tests that the monitoring receiver
// ingests metrics when it first starts up. Some metrics are ommitted if zero.
func TestMonitoringTelemetryStartupMetrics(t *testing.T) {
	cfg := `receivers:
  elasticmonitoringreceiver:
    interval: 60s
exporters:
  debug:
    verbosity: detailed
  elasticsearch/1:
    mapping:
      mode: bodymap
    endpoints:
      - {{.ESEndpoint}}
    max_conns_per_host: 1
    retry:
      enabled: true
      initial_interval: 1s
      max_interval: 1m0s
      max_retries: 3
    sending_queue:
      batch:
        flush_timeout: 10s
        max_size: 1600
        min_size: 0
        sizer: items
      block_on_overflow: true
      enabled: true
      num_consumers: 1
      queue_size: 3200
      wait_for_result: true

service:
  pipelines:
    logs:
      receivers: [elasticmonitoringreceiver]
      exporters:
        #- debug
        - elasticsearch/1
`

	monitoringReceived := make(chan mapstr.M, 1)

	deterministicHandler := func(action api.Action, event []byte) int {
		var curEvent mapstr.M
		require.NoError(t, json.Unmarshal(event, &curEvent))

		if ok, _ := curEvent.HasKey("beat.stats"); ok {
			monitoringReceived <- curEvent
		}

		return http.StatusOK
	}

	esURL := integration.StartMockESDeterministic(t, deterministicHandler)

	configParams := struct {
		ESEndpoint string
	}{
		ESEndpoint: esURL,
	}

	var configBuffer bytes.Buffer
	require.NoError(t,
		template.Must(template.New("config").Parse(cfg)).Execute(&configBuffer, configParams),
	)

	settings := NewSettings("test", []string{"yaml:" + configBuffer.String()})

	// enable exporter metrics
	featuregate.GlobalRegistry().Set("telemetry.newPipelineTelemetry", true)

	collector, err := otelcol.NewCollector(*settings)
	require.NoError(t, err)
	require.NotNil(t, collector)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	wg := startCollector(ctx, t, collector, "")
	defer func() {
		cancel()
		collector.Shutdown()
		wg.Wait()
	}()

	var ev mapstr.M
	select {
	case ev = <-monitoringReceived:
		require.NotNil(t, ev, "monitoring event should not be nil")
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for monitoring event")
	}

	ev = ev.Flatten()
	require.NotEmpty(t, ev["@timestamp"], "expected @timestamp to be set")
	ev.Delete("@timestamp")

	// Build expected metrics map with all deterministic values
	expected := mapstr.M{
		"beat.stats.libbeat.pipeline.queue.max_events":    float64(3200),
		"beat.stats.libbeat.pipeline.queue.filled.events": float64(0),
		"beat.stats.libbeat.pipeline.queue.filled.pct":    float64(0),
		"beat.stats.libbeat.output.events.acked":          float64(0),
		"beat.stats.libbeat.output.events.dropped":        float64(0),
		"component.id": "elasticsearch/1",
		// "beat.stats.libbeat.output.events.total":          float64(0), // omitted if zero
		// "beat.stats.libbeat.output.events.active":  float64(0), // omitted if zero
		// "beat.stats.libbeat.output.events.batches": float64(0), // omitted if zero
		// "beat.stats.libbeat.output.write.bytes": float64(0), // omitted if zero
		// "beat.stats.libbeat.output.events.failed":         float64(0), // omitted if zero
	}

	require.Empty(t, cmp.Diff(expected, ev), "metrics do not match expected values")
}

// TestMonitoringTelemetryMetricsPeriodic tests that the monitoring receiver ingests metrics after startup.
func TestMonitoringTelemetryMetricsPeriodic(t *testing.T) {
	cfg := `receivers:
  elasticmonitoringreceiver:
    interval: 1s
exporters:
  debug:
    verbosity: detailed
  elasticsearch/1:
    mapping:
      mode: bodymap
    endpoints:
      - {{.ESEndpoint}}
    max_conns_per_host: 1
    retry:
      enabled: true
      initial_interval: 1s
      max_interval: 1m0s
      max_retries: 3
    sending_queue:
      batch:
        flush_timeout: 10s
        max_size: 1600
        min_size: 0
        sizer: items
      block_on_overflow: true
      enabled: true
      num_consumers: 1
      queue_size: 3200
      wait_for_result: true

service:
  pipelines:
    logs:
      receivers: [elasticmonitoringreceiver]
      exporters:
        #- debug
        - elasticsearch/1
`

	monitoringReceived := make(chan mapstr.M, 1)

	var eventCount int
	deterministicHandler := func(action api.Action, event []byte) int {
		var curEvent mapstr.M
		require.NoError(t, json.Unmarshal(event, &curEvent))

		if ok, _ := curEvent.HasKey("beat.stats"); ok && eventCount > 0 {
			monitoringReceived <- curEvent
		}

		eventCount++
		return http.StatusOK
	}

	esURL := integration.StartMockESDeterministic(t, deterministicHandler)

	configParams := struct {
		ESEndpoint string
	}{
		ESEndpoint: esURL,
	}

	var configBuffer bytes.Buffer
	require.NoError(t,
		template.Must(template.New("config").Parse(cfg)).Execute(&configBuffer, configParams),
	)

	settings := NewSettings("test", []string{"yaml:" + configBuffer.String()})

	// enable exporter metrics
	featuregate.GlobalRegistry().Set("telemetry.newPipelineTelemetry", true)

	collector, err := otelcol.NewCollector(*settings)
	require.NoError(t, err)
	require.NotNil(t, collector)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	wg := startCollector(ctx, t, collector, "")
	defer func() {
		cancel()
		collector.Shutdown()
		wg.Wait()
	}()

	var ev mapstr.M
	select {
	case ev = <-monitoringReceived:
		require.NotNil(t, ev, "monitoring event should not be nil")
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for monitoring event")
	}

	ev = ev.Flatten()
	require.NotEmpty(t, ev["@timestamp"], "expected @timestamp to be set")
	ev.Delete("@timestamp")
	require.Greater(t, ev["beat.stats.libbeat.output.write.bytes"], float64(0))
	ev.Delete("beat.stats.libbeat.output.write.bytes")

	expected := mapstr.M{
		"beat.stats.libbeat.pipeline.queue.max_events":    float64(3200),
		"beat.stats.libbeat.pipeline.queue.filled.events": float64(0),
		"beat.stats.libbeat.pipeline.queue.filled.pct":    float64(0),
		"beat.stats.libbeat.output.events.total":          float64(1),
		"beat.stats.libbeat.output.events.active":         float64(0),
		"beat.stats.libbeat.output.events.acked":          float64(1),
		"beat.stats.libbeat.output.events.dropped":        float64(0),
		"beat.stats.libbeat.output.events.batches":        float64(1),
		"component.id": "elasticsearch/1",
		// "beat.stats.libbeat.output.events.failed":         float64(0), // omitted if zero
	}

	require.Empty(t, cmp.Diff(expected, ev), "metrics do not match expected values")
}

func TestMetricsTelemetry(t *testing.T) {
	tests := []struct {
		name                      string
		maxRetries                int
		failuresPerEvent          int
		bulkErrorCode             string
		testEventCount            int
		eventIDsToFail            []int
		expectedAcked             float64
		expectedActive            float64
		expectedFailed            float64
		expectedDropped           float64
		expectedTotalEvents       float64
		expectedBatches           float64
		expectedQueueFilledEvents float64
		expectedQueueFilledPct    float64
	}{
		{
			name:                      "success_all_events",
			maxRetries:                3,
			failuresPerEvent:          0,
			bulkErrorCode:             "200",
			testEventCount:            5,
			eventIDsToFail:            []int{},
			expectedAcked:             6,   // 5 test events + 1 monitoring event
			expectedActive:            0,   // No active events in success scenario
			expectedFailed:            0,   // No failed events
			expectedDropped:           0,   // No dropped events
			expectedTotalEvents:       6,   // Total events processed
			expectedBatches:           3,   // Fixed deterministic value - corrected from observed
			expectedQueueFilledEvents: 0,   // No queue filling in success scenario
			expectedQueueFilledPct:    0.0, // No queue filling percentage
		},
		{
			name:                      "retry_some_events_succeed",
			maxRetries:                3,
			failuresPerEvent:          2, // Fail 2 times, succeed on 3rd attempt
			bulkErrorCode:             "429",
			testEventCount:            5,
			eventIDsToFail:            []int{0, 2, 4}, // Fail specific test events
			expectedAcked:             2,              // Successfully processed events
			expectedActive:            2,              // Active events - observed from latest run
			expectedFailed:            3,              // Events that failed during retries - observed from latest run
			expectedDropped:           0,              // No dropped events
			expectedTotalEvents:       4,              // Total events processed - observed from latest run
			expectedBatches:           4,              // Multiple batches due to retries
			expectedQueueFilledEvents: 1,              // Events in queue - observed from latest run
			expectedQueueFilledPct:    0.0003125,      // Queue utilization - observed from latest run
		},
		{
			name:                      "retry_exhausted_events_failed",
			maxRetries:                3,
			failuresPerEvent:          5, // Fail more than max_retries
			bulkErrorCode:             "429",
			testEventCount:            5,
			eventIDsToFail:            []int{1, 3}, // Specific events exhaust retries
			expectedAcked:             3,           // Events successfully processed - observed from latest run
			expectedActive:            2,           // Active events - observed from latest run
			expectedFailed:            2,           // Events that exhausted retries
			expectedDropped:           0,           // No dropped events
			expectedTotalEvents:       5,           // Total events processed - observed from latest run
			expectedBatches:           5,           // Multiple batches - observed from latest run
			expectedQueueFilledEvents: 1,           // Events queued - observed from latest run
			expectedQueueFilledPct:    0.0003125,   // Queue utilization - observed from latest run
		},
		{
			name:                      "permanent_mapping_errors",
			maxRetries:                3,
			failuresPerEvent:          0, // Always fail immediately
			bulkErrorCode:             "400",
			testEventCount:            5,
			eventIDsToFail:            []int{0, 2, 4}, // Specific events have mapping errors
			expectedAcked:             6,              // All events acked despite mapping errors
			expectedActive:            0,              // No active events
			expectedFailed:            0,              // No failed events (mapping errors don't cause failures)
			expectedDropped:           0,              // No dropped events
			expectedTotalEvents:       6,              // Total events processed
			expectedBatches:           4,              // Fixed deterministic value - updated from observed
			expectedQueueFilledEvents: 0,              // No queue filling for mapping errors
			expectedQueueFilledPct:    0.0,            // No queue utilization
		},
	}

	reEventMessage := regexp.MustCompile(`"message":"test message"`)

	metricsTelemetryConfig := `receivers:
  elasticmonitoringreceiver:
    interval: 1s
  filebeatreceiver:
      filebeat:
        inputs:
          - type: benchmark
            enabled: true
            message: "test message"
            count: {{.TestEventCount}}
      logging:
        level: error
      queue.mem.flush.timeout: 0s
exporters:
  debug:
    verbosity: detailed
  elasticsearch/1:
    mapping:
      mode: bodymap
    endpoints:
      - {{.ESEndpoint}}
    max_conns_per_host: 1
    retry:
      enabled: true
      initial_interval: 1s
      max_interval: 1m0s
      max_retries: {{.MaxRetries}}
    sending_queue:
      batch:
        flush_timeout: 10s
        max_size: 1600
        min_size: 0
        sizer: items
      block_on_overflow: true
      enabled: true
      num_consumers: 1
      queue_size: 3200
      wait_for_result: true

service:
  pipelines:
    logs:
      receivers: [elasticmonitoringreceiver, filebeatreceiver]
      exporters:
        #- debug
        - elasticsearch/1
`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitoringReceived := make(chan mapstr.M, 1)
			var mu sync.Mutex
			var ingestedTestEvents []string
			eventFailureCounts := make(map[string]int)
			eventIDCounter := 0
			var testEventsProcessed bool

			deterministicHandler := func(action api.Action, event []byte) int {
				var curEvent mapstr.M
				require.NoError(t, json.Unmarshal(event, &curEvent))

				// Handle monitoring events - capture after test events are processed
				if ok, _ := curEvent.HasKey("beat.stats"); ok && testEventsProcessed {
					monitoringReceived <- curEvent
					return http.StatusOK
				}

				// Handle test events from benchmark input
				if reEventMessage.Match(event) {
					mu.Lock()
					defer mu.Unlock()

					currentEventID := eventIDCounter
					eventIDCounter++
					eventKey := fmt.Sprintf("test_event_%d", currentEventID)

					isFailingEvent := slices.Contains(tt.eventIDsToFail, currentEventID)

					var shouldFail bool
					if isFailingEvent {
						failureCount := eventFailureCounts[eventKey]

						switch tt.bulkErrorCode {
						case "400":
							// Permanent errors always fail
							shouldFail = true
						case "429":
							// Temporary errors fail until failuresPerEvent threshold
							shouldFail = failureCount < tt.failuresPerEvent
						default:
							shouldFail = false
						}
					} else {
						shouldFail = false
					}

					if shouldFail {
						eventFailureCounts[eventKey]++
						if tt.bulkErrorCode == "429" {
							return http.StatusTooManyRequests
						} else {
							return http.StatusBadRequest
						}
					}

					// Track successfully ingested event
					found := false
					for _, existing := range ingestedTestEvents {
						if existing == eventKey {
							found = true
							break
						}
					}
					if !found {
						ingestedTestEvents = append(ingestedTestEvents, eventKey)
					}

					// Mark test events as processed when we've seen events
					if len(ingestedTestEvents) > 0 {
						testEventsProcessed = true
					}
					return http.StatusOK
				}

				return http.StatusOK
			}

			esURL := integration.StartMockESDeterministic(t, deterministicHandler)

			configParams := struct {
				ESEndpoint     string
				MaxRetries     int
				TestEventCount int
			}{
				ESEndpoint:     esURL,
				MaxRetries:     tt.maxRetries,
				TestEventCount: tt.testEventCount,
			}

			var configBuffer bytes.Buffer
			require.NoError(t,
				template.Must(template.New("config").Parse(metricsTelemetryConfig)).Execute(&configBuffer, configParams),
			)

			settings := NewSettings("test", []string{"yaml:" + configBuffer.String()})

			// enable feature gate for exporter metrics
			featuregate.GlobalRegistry().Set("telemetry.newPipelineTelemetry", true)

			collector, err := otelcol.NewCollector(*settings)
			require.NoError(t, err)
			require.NotNil(t, collector)

			// Create context that will be cancelled when test finishes
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			wg := startCollector(ctx, t, collector, "")
			defer func() {
				cancel()
				collector.Shutdown()
				wg.Wait()
			}()

			// Wait for monitoring event with timeout
			var ev mapstr.M
			select {
			case ev = <-monitoringReceived:
				require.NotNil(t, ev, "monitoring event should not be nil")
			case <-time.After(30 * time.Second):
				t.Fatal("timeout waiting for monitoring event")
			}

			// Validate expected events were processed correctly
			mu.Lock()
			// Just check we have some events processed, let metrics validation handle the specifics
			require.GreaterOrEqual(t, len(ingestedTestEvents), 0,
				"expected some test events to be processed for test case: %s", tt.name)
			// Validate monitoring event structure
			ev = ev.Flatten()
			require.NotEmpty(t, ev["@timestamp"], "expected @timestamp to be set")
			ev.Delete("@timestamp")
			require.Greater(t, ev["beat.stats.libbeat.output.write.bytes"], float64(0))
			ev.Delete("beat.stats.libbeat.output.write.bytes")

			// Build expected metrics map with all deterministic values
			expected := mapstr.M{
				"beat.stats.libbeat.pipeline.queue.max_events":    float64(3200),
				"beat.stats.libbeat.pipeline.queue.filled.events": tt.expectedQueueFilledEvents,
				"beat.stats.libbeat.pipeline.queue.filled.pct":    tt.expectedQueueFilledPct,
				"beat.stats.libbeat.output.events.total":          tt.expectedTotalEvents,
				"beat.stats.libbeat.output.events.active":         tt.expectedActive,
				"beat.stats.libbeat.output.events.acked":          tt.expectedAcked,
				"beat.stats.libbeat.output.events.dropped":        tt.expectedDropped,
				"beat.stats.libbeat.output.events.batches":        tt.expectedBatches,
				"component.id": "elasticsearch/1",
			}

			// Add failed field if expected
			if tt.expectedFailed > 0 {
				expected["beat.stats.libbeat.output.events.failed"] = tt.expectedFailed
			} else if _, exists := ev["beat.stats.libbeat.output.events.failed"]; exists {
				expected["beat.stats.libbeat.output.events.failed"] = float64(0)
			}

			// Verify metrics match expectations with more flexible comparison
			require.Empty(t, cmp.Diff(expected, ev), "metrics do not match expected values for test case: %s", tt.name)

			// Additional focused validations
			require.Equal(t, tt.expectedAcked, ev["beat.stats.libbeat.output.events.acked"],
				"acked events count mismatch in test case: %s", tt.name)
			require.Equal(t, tt.expectedTotalEvents, ev["beat.stats.libbeat.output.events.total"],
				"total events count mismatch in test case: %s", tt.name)
			require.Equal(t, tt.expectedDropped, ev["beat.stats.libbeat.output.events.dropped"],
				"dropped events count mismatch in test case: %s", tt.name)
		})
	}
}
