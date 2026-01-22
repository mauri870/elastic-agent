// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcol

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/elastic/elastic-agent/testing/integration"
	"github.com/elastic/mock-es/pkg/api"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
)

// This test verifies that the collector exposes the required otelcol and exporter metrics for traces, metrics and logs.
// It serves as a safeguard to detect if any of these metrics are removed or renamed upstream.
func TestInternalMonitoringLogsMetrics(t *testing.T) {
	cfg := `receivers:
  elasticmonitoringreceiver:
    interval: 3s
  loadgen:
    concurrency: 1
    metrics:
      max_replay: 0
    traces:
      max_replay: 0
    logs:
      max_replay: 0
exporters:
  debug:
    verbosity: detailed
  elasticsearch/telemetry:
    endpoints:
      - {{.ESTelemetry}}
    max_conns_per_host: 1
    retry:
      enabled: true
      initial_interval: 1s
      max_interval: 1m0s
      max_retries: 1
    sending_queue:
      batch:
        flush_timeout: 10s
        max_size: 1
        min_size: 0
        sizer: items
      block_on_overflow: true
      enabled: true
      num_consumers: 1
      queue_size: 3200
      wait_for_result: true
  elasticsearch/monitoring:
    mapping:
      mode: bodymap
    endpoints:
      - {{.ESMonitoring}}
    max_conns_per_host: 1
    retry:
      enabled: true
      initial_interval: 1s
      max_interval: 1m0s
      max_retries: 1 
    sending_queue:
      batch:
        flush_timeout: 10s
        max_size: 1
        min_size: 0
        sizer: items
      block_on_overflow: true
      enabled: true
      num_consumers: 1
      queue_size: 3200
      wait_for_result: true

service:
  pipelines:
    logs/monitoring:
      receivers: [elasticmonitoringreceiver]
      exporters:
        - elasticsearch/monitoring
    #metrics:
    #  receivers: [loadgen]
    #  exporters:
    #    - elasticsearch/telemetry
    #traces:
    #  receivers: [loadgen]
    #  exporters:
    #    - elasticsearch/telemetry
    logs:
      receivers: [loadgen]
      exporters:
        - elasticsearch/telemetry
`
	var done atomic.Bool

	// keep track of logs, traces and metrics separately
	// inside handler: identify which kind of event it is. Reply with a success, retry or failed status based on counts
	// Make sure we trigger all cases required for the metrics expectations
	var logsCount int
	var tracesCount int
	var metricsCount int
	telemetryHandler := func(action api.Action, event []byte) int {
		var curEvent mapstr.M
		require.NoError(t, json.Unmarshal(event, &curEvent))

		if done.Load() {
			return http.StatusOK
		}

		dsValue, err := curEvent.GetValue("data_stream.type")
		assert.NoError(t, err, "data_stream.type must be present in event")
		ds := dsValue.(string)
		switch ds {
		case "metrics":
			metricsCount++
		case "traces":
			tracesCount++
		case "logs":
			logsCount++
		default:
			t.Fatal("data_stream.type not handled:", ds)
		}

		return http.StatusOK
	}

	esTelemetryURL := integration.StartMockESDeterministic(t, telemetryHandler)

	var monitoringCount int
	monitoringReceived := make(chan map[string]mapstr.M, 1)
	monitoringMetrics := make(map[string]mapstr.M)
	exporterSeen := make(map[string]bool)
	monitoringHandler := func(action api.Action, event []byte) int {
		var ev mapstr.M
		require.NoError(t, json.Unmarshal(event, &ev))
		ev = ev.Flatten()

		exporterID := ev["component.id"].(string)

		// skip startup metrics for each receiver
		if !exporterSeen[exporterID] {
			exporterSeen[exporterID] = true
			return http.StatusOK
		}
		monitoringMetrics[exporterID] = ev

		if len(monitoringMetrics) == 2 {
			monitoringReceived <- monitoringMetrics
			done.Store(true)
		}

		return http.StatusOK
	}

	esMonitoringURL := integration.StartMockESDeterministic(t, monitoringHandler)

	configParams := struct {
		ESTelemetry  string
		ESMonitoring string
	}{
		ESTelemetry:  esTelemetryURL,
		ESMonitoring: esMonitoringURL,
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

	var metrics map[string]mapstr.M
	select {
	case metrics = <-monitoringReceived:
		require.NotNil(t, metrics, "monitoring event should not be nil")
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for monitoring event")
	}

	assert.Len(t, metrics, 2, "expected monitoring metrics for all exporters")
	// check for telemetry metrics
	ev := monitoringMetrics["elasticsearch/telemetry"]

	require.NotEmpty(t, ev["@timestamp"], "expected @timestamp to be set")
	ev.Delete("@timestamp")
	require.Greater(t, ev["beat.stats.libbeat.output.write.bytes"], float64(0))
	ev.Delete("beat.stats.libbeat.output.write.bytes")

	expected := mapstr.M{
		"beat.stats.libbeat.pipeline.queue.max_events":    float64(3200),
		"beat.stats.libbeat.pipeline.queue.filled.events": float64(1),
		"beat.stats.libbeat.pipeline.queue.filled.pct":    float64(0.0003125),
		"beat.stats.libbeat.output.events.total":          float64(logsCount + tracesCount + metricsCount + monitoringCount),
		"beat.stats.libbeat.output.events.active":         float64(0),
		"beat.stats.libbeat.output.events.acked":          float64(logsCount + tracesCount + metricsCount + monitoringCount),
		"beat.stats.libbeat.output.events.dropped":        float64(0),
		"beat.stats.libbeat.output.events.batches":        float64(logsCount + tracesCount + metricsCount + monitoringCount),
		"component.id": "elasticsearch/telemetry",
		// "beat.stats.libbeat.output.events.failed":         float64(0), // omitted if zero
	}

	require.Empty(t, cmp.Diff(expected, ev), "metrics do not match expected values")
}
