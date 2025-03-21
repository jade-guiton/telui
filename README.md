# TelUI

A basic in-memory OpenTelemetry traces/logs/metrics receiver and viewer.

```
Usage of ./telui:
  -grpc int
        Port for OTLP/gRPC server (0 to disable) (default 4317)
  -http int
        Port for OTLP/HTTP server (0 to disable) (default 4318)
  -ui int
        Port for web interface (default 8080)
  -verbose
        Log incoming data
```

## Screenshots

![screenshot of traces tab](doc/screenshot-traces.png)

![screenshot of logs tab](doc/screenshot-logs.png)

![screenshot of metrics tab](doc/screenshot-metrics.png)

