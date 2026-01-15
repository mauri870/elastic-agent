// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent/testing/integration"
	"github.com/elastic/mock-es/pkg/api"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
)

var metricsTelemetryConfig = `receivers:
  elasticmonitoringreceiver:
    interval: 1s
  filebeatreceiver:
      filebeat:
        inputs:
          - type: benchmark
            enabled: true
            message: "test message"
            count: 1
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
      - %s
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
      receivers: [elasticmonitoringreceiver, filebeatreceiver]
      exporters:
        #- debug
        - elasticsearch/1
`

func TestMetricsTelemetry(t *testing.T) {
	tests := []struct {
		name                string
		mockESHTTPStatus    int
		expectedAcked       float64
		expectedFailed      float64
		expectedDropped     float64
		expectedTotalEvents float64
		expectedBatches     float64
		expectFailedField   bool
	}{
		{
			name:                "success",
			mockESHTTPStatus:    http.StatusOK,
			expectedAcked:       2,
			expectedFailed:      0,
			expectedDropped:     0,
			expectedTotalEvents: 2,
			expectedBatches:     2,
			expectFailedField:   false,
		},
		{
			name:                "failure",
			mockESHTTPStatus:    http.StatusInternalServerError,
			expectedAcked:       2, // both events still get acked even when ES returns error
			expectedFailed:      0, // the failure is handled differently at this level
			expectedDropped:     0,
			expectedTotalEvents: 2,
			expectedBatches:     2,
			expectFailedField:   false,
		},
		{
			name:                "retry_exhausted_events_failed",
			mockESHTTPStatus:    http.StatusTooManyRequests, // 429 - retryable but will exhaust retries
			expectedAcked:       1,                          // only monitoring event gets acked
			expectedFailed:      2,                          // after max retries, events are failed, not dropped
			expectedDropped:     0,                          // events are not dropped in this scenario
			expectedTotalEvents: 1,                          // total successful events published
			expectedBatches:     3,                          // more batches due to retries
			expectFailedField:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitoringReceived := make(chan mapstr.M, 1)
			var logIngested bool

			deterministicHandler := func(action api.Action, event []byte) int {
				var curEvent mapstr.M
				require.NoError(t, json.Unmarshal(event, &curEvent))

				// detect log events
				if ok, _ := curEvent.HasKey("input.type"); ok {
					logIngested = true
					return tt.mockESHTTPStatus
				}

				// wait for log ingested and then capture the latest monitoring event.
				if ok, _ := curEvent.HasKey("beat.stats"); ok && logIngested {
					select {
					case monitoringReceived <- curEvent:
						// Successfully sent monitoring event
					default:
						// Channel is full, skip this event
					}
				}

				return http.StatusOK
			}

			esURL := integration.StartMockESDeterministic(t, deterministicHandler)
			settings := NewSettings("test", []string{"yaml:" + fmt.Sprintf(metricsTelemetryConfig, esURL)})

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
			case <-time.After(20 * time.Second):
				t.Fatal("timeout waiting for monitoring event")
			}

			// Validate monitoring event structure
			ev = ev.Flatten()
			require.NotEmpty(t, ev["@timestamp"], "expected @timestamp to be set")
			ev.Delete("@timestamp")
			require.Greater(t, ev["beat.stats.libbeat.output.write.bytes"], float64(0))
			ev.Delete("beat.stats.libbeat.output.write.bytes")

			// Build expected metrics map
			expected := mapstr.M{
				"beat.stats.libbeat.pipeline.queue.filled.events": float64(1), // Events stay in queue during retries
				"beat.stats.libbeat.pipeline.queue.max_events":    float64(3200),
				"beat.stats.libbeat.pipeline.queue.filled.pct":    float64(0.0003125),
				"beat.stats.libbeat.output.events.total":          tt.expectedTotalEvents,
				"beat.stats.libbeat.output.events.active":         float64(0),
				"beat.stats.libbeat.output.events.acked":          tt.expectedAcked,
				"beat.stats.libbeat.output.events.dropped":        tt.expectedDropped,
				"beat.stats.libbeat.output.events.batches":        tt.expectedBatches,
				"component.id": "elasticsearch/1",
			}

			// Special handling for retry test case which has different pipeline queue metrics
			if tt.name == "retry_exhausted_events_failed" {
				// Events remain in the pipeline queue during retries
				expected["beat.stats.libbeat.pipeline.queue.filled.events"] = float64(1)
				expected["beat.stats.libbeat.pipeline.queue.filled.pct"] = float64(0.0003125)
			} else {
				// For success and failure cases, queue should be empty
				expected["beat.stats.libbeat.pipeline.queue.filled.events"] = float64(0)
				expected["beat.stats.libbeat.pipeline.queue.filled.pct"] = float64(0)
			}

			// Conditionally add failed field based on test expectations
			if tt.expectFailedField || tt.expectedFailed > 0 {
				expected["beat.stats.libbeat.output.events.failed"] = tt.expectedFailed
			} else if _, exists := ev["beat.stats.libbeat.output.events.failed"]; exists {
				expected["beat.stats.libbeat.output.events.failed"] = tt.expectedFailed
			}

			// Verify metrics match expectations
			require.Empty(t, cmp.Diff(expected, ev), "metrics do not match expected values for test case: %s", tt.name)

			// Additional validations
			require.Equal(t, tt.expectedAcked, ev["beat.stats.libbeat.output.events.acked"], "acked events count mismatch")
			require.Equal(t, tt.expectedTotalEvents, ev["beat.stats.libbeat.output.events.total"], "total events count mismatch")
			require.Equal(t, tt.expectedDropped, ev["beat.stats.libbeat.output.events.dropped"], "dropped events count mismatch")
		})
	}
}
