# Elastic Distribution for OpenTelemetry Collector

This is an Elastic supported distribution of the [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector).

## Running the Elastic Distribution for OpenTelemetry Collector

To run the Elastic Distribution for OpenTelemetry Collector you can use Elastic-Agent binary downloaded for your OS and architecture.
Running command

```bash
./elastic-agent otel --config otel.yml
```

from unpacked Elastic Agent package will run Elastic-Agent as an OpenTelemetry Collector. The `--config` flag needs to point to [OpenTelemetry Collector Configuration file](https://opentelemetry.io/docs/collector/configuration/). OTel mode is available only using `otel` subcommand. Elastic Agent will not do any autodetection of configuration file passed when used without `otel` subcommand and will try to run normally.

To validate OTel configuration run `otel validate` subcommand:

```bash
./elastic-agent otel validate --config otel.yml
```

[feature gates](https://github.com/open-telemetry/opentelemetry-collector/blob/main/featuregate/README.md#controlling-gates) are supported using `--feature-gates` flag.

## Components

This section provides a summary of components included in the Elastic Distribution for OpenTelemetry Collector.

### Receivers

| Component | Version |
|---|---|
| [jaegerreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/jaegerreceiver/v0.111.0/receiver/jaegerreceiver/README.md) | v0.111.0 |
| [prometheusreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/prometheusreceiver/v0.111.0/receiver/prometheusreceiver/README.md) | v0.111.0 |
| [zipkinreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/zipkinreceiver/v0.111.0/receiver/zipkinreceiver/README.md) | v0.111.0 |
| [filelogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/filelogreceiver/v0.111.0/receiver/filelogreceiver/README.md) | v0.111.0 |
| [hostmetricsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/hostmetricsreceiver/v0.111.0/receiver/hostmetricsreceiver/README.md) | v0.111.0 |
| [httpcheckreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/httpcheckreceiver/v0.111.0/receiver/httpcheckreceiver/README.md) | v0.111.0 |
| [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/k8sclusterreceiver/v0.111.0/receiver/k8sclusterreceiver/README.md) | v0.111.0 |
| [k8sobjectsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/k8sobjectsreceiver/v0.111.0/receiver/k8sobjectsreceiver/README.md) | v0.111.0 |
| [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/receiver/kubeletstatsreceiver/v0.111.0/receiver/kubeletstatsreceiver/README.md) | v0.111.0 |
| [otlpreceiver](https://github.com/open-telemetry/opentelemetry-collector/blob/receiver/otlpreceiver/v0.111.0/receiver/otlpreceiver/README.md) | v0.111.0 |

### Exporters

| Component | Version |
|---|---|
| [elasticsearchexporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/exporter/elasticsearchexporter/v0.111.0/exporter/elasticsearchexporter/README.md) | v0.111.0 |
| [fileexporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/exporter/fileexporter/v0.111.0/exporter/fileexporter/README.md) | v0.111.0 |
| [debugexporter](https://github.com/open-telemetry/opentelemetry-collector/blob/exporter/debugexporter/v0.111.0/exporter/debugexporter/README.md) | v0.111.0 |
| [otlpexporter](https://github.com/open-telemetry/opentelemetry-collector/blob/exporter/otlpexporter/v0.111.0/exporter/otlpexporter/README.md) | v0.111.0 |
| [otlphttpexporter](https://github.com/open-telemetry/opentelemetry-collector/blob/exporter/otlphttpexporter/v0.111.0/exporter/otlphttpexporter/README.md) | v0.111.0 |

### Processors

| Component | Version |
|---|---|
| [elasticinframetricsprocessor](https://github.com/elastic/opentelemetry-collector-components/blob/processor/elasticinframetricsprocessor/v0.12.0/processor/elasticinframetricsprocessor/README.md) | v0.12.0 |
| [elastictraceprocessor](https://github.com/elastic/opentelemetry-collector-components/blob/processor/elastictraceprocessor/v0.2.1/processor/elastictraceprocessor/README.md) | v0.2.1 |
| [lsmintervalprocessor](https://github.com/elastic/opentelemetry-collector-components/blob/processor/lsmintervalprocessor/v0.2.0/processor/lsmintervalprocessor/README.md) | v0.2.0 |
| [memorylimiterprocessor](https://github.com/open-telemetry/opentelemetry-collector/blob/processor/memorylimiterprocessor/v0.111.0/processor/memorylimiterprocessor/README.md) | v0.111.0 |
| [attributesprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/processor/attributesprocessor/v0.111.0/processor/attributesprocessor/README.md) | v0.111.0 |
| [filterprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/processor/filterprocessor/v0.111.0/processor/filterprocessor/README.md) | v0.111.0 |
| [k8sattributesprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/processor/k8sattributesprocessor/v0.111.0/processor/k8sattributesprocessor/README.md) | v0.111.0 |
| [resourcedetectionprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/processor/resourcedetectionprocessor/v0.111.0/processor/resourcedetectionprocessor/README.md) | v0.111.0 |
| [resourceprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/processor/resourceprocessor/v0.111.0/processor/resourceprocessor/README.md) | v0.111.0 |
| [transformprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/processor/transformprocessor/v0.111.0/processor/transformprocessor/README.md) | v0.111.0 |
| [batchprocessor](https://github.com/open-telemetry/opentelemetry-collector/blob/processor/batchprocessor/v0.111.0/processor/batchprocessor/README.md) | v0.111.0 |

### Extensions

| Component | Version |
|---|---|
| [healthcheckextension](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/extension/healthcheckextension/v0.111.0/extension/healthcheckextension/README.md) | v0.111.0 |
| [pprofextension](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/extension/pprofextension/v0.111.0/extension/pprofextension/README.md) | v0.111.0 |
| [filestorage](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/extension/storage/filestorage/v0.111.0/extension/storage/filestorage/README.md) | v0.111.0 |
| [memorylimiterextension](https://github.com/open-telemetry/opentelemetry-collector/blob/extension/memorylimiterextension/v0.111.0/extension/memorylimiterextension/README.md) | v0.111.0 |

### Connectors

| Component | Version |
|---|---|
| [signaltometricsconnector](https://github.com/elastic/opentelemetry-collector-components/blob/connector/signaltometricsconnector/v0.2.1/connector/signaltometricsconnector/README.md) | v0.2.1 |
| [spanmetricsconnector](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/connector/spanmetricsconnector/v0.111.0/connector/spanmetricsconnector/README.md) | v0.111.0 |
