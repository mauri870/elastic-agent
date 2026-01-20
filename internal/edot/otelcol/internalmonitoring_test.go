// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcol

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"text/template"
	"time"

	"github.com/elastic/elastic-agent/internal/pkg/otel/internaltelemetry"
	"github.com/elastic/elastic-agent/testing/integration"
	"github.com/elastic/mock-es/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol"
)

// This test verifies that the collector exposes the required otelcol and exporter metrics.
// It serves as a safeguard to detect if any of these metrics are removed or renamed upstream.
func TestInternalMonitoringLogsMetrics(t *testing.T) {
	cfg := `receivers:
  filebeatreceiver:
      filebeat:
        inputs:
          - type: benchmark
            enabled: true
            message: "test message"
            count: 10
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
      max_retries: 1 
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
      receivers: [filebeatreceiver]
      exporters:
        #- debug
        - elasticsearch/1
`

	var eventCount int
	deterministicHandler := func(action api.Action, event []byte) int {
		eventCount++

		switch {
		case eventCount <= 2:
			return http.StatusTooManyRequests
		case eventCount <= 5:
			return http.StatusBadRequest
		default:
			return http.StatusOK
		}
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

	expectedMetrics := []string{
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		// needs traces data to be generated
		// "otelcol_exporter_sent_spans",
		// "otelcol_exporter_send_failed_spans",
		"otelcol_exporter_sent_log_records",
		"otelcol_exporter_send_failed_log_records",
		// needs metrics data to be generated
		// "otelcol_exporter_sent_metric_points",
		// "otelcol_exporter_send_failed_metric_points",
		"otelcol.elasticsearch.docs.processed",
		"otelcol.elasticsearch.docs.retried",
		"otelcol.elasticsearch.bulk_requests.count",
		"otelcol.elasticsearch.flushed.bytes",
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		metrics, err := internaltelemetry.ReadMetrics(ctx)
		if !assert.NoError(c, err) {
			return
		}

		foundMetrics := make(map[string]bool)
		for _, sm := range metrics.ScopeMetrics {
			for _, met := range sm.Metrics {
				foundMetrics[met.Name] = true
			}
		}

		if len(foundMetrics) == 0 {
			t.Log("No metrics found")
			return
		}

		for _, expectedMetric := range expectedMetrics {
			assert.True(c, foundMetrics[expectedMetric], "Expected metric %s not found", expectedMetric)
		}
	}, 30*time.Second, 1*time.Second, "All expected metrics should be present")
}
