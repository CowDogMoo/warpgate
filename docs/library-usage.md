# Using Warpgate as a Go Library

Warpgate can be imported as a Go module to build container images and AWS AMIs
programmatically. This guide covers the public API with working examples.

## Installation

```bash
go get github.com/cowdogmoo/warpgate/v3@latest
```

## Package Overview

| Package            | Purpose                                          |
| ------------------ | ------------------------------------------------ |
| `builder`          | Core interfaces, `BuildService`, `Config`        |
| `builder/buildkit` | BuildKit-based container image builder           |
| `builder/ami`      | AMI builder, `MonitorConfig`, `StatusCallback`   |
| `progress`         | Reusable multi-line progress bar display         |
| `config`           | Global configuration loading                     |
| `templates`        | Template discovery and loading                   |

## Container Builds

### Basic Build

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cowdogmoo/warpgate/v3/builder"
    "github.com/cowdogmoo/warpgate/v3/builder/buildkit"
    "github.com/cowdogmoo/warpgate/v3/config"
)

func main() {
    ctx := context.Background()

    // Load global config (optional — uses defaults if no config file).
    globalCfg, err := config.Load()
    if err != nil && !config.IsNotFoundError(err) {
        log.Fatal(err)
    }

    // Create the build service with a BuildKit builder factory.
    service := builder.NewBuildService(globalCfg, func(ctx context.Context) (builder.ContainerBuilder, error) {
        return buildkit.NewBuildKitBuilder(ctx)
    })

    // Define the build configuration.
    buildConfig := builder.Config{
        Name: "my-image",
        Base: builder.BaseImage{Image: "ubuntu:22.04"},
        Provisioners: []builder.Provisioner{
            {Type: "shell", Inline: []string{"apt-get update", "apt-get install -y curl"}},
        },
    }

    opts := builder.BuildOptions{
        TargetType:    "container",
        Architectures: []string{"amd64"},
        Registry:      "ghcr.io/myorg",
    }

    results, err := service.ExecuteContainerBuild(ctx, buildConfig, opts)
    if err != nil {
        log.Fatal(err)
    }

    for _, r := range results {
        fmt.Printf("Built: %s (%s)\n", r.ImageRef, r.Duration)
    }
}
```

### Dockerfile Build

If your template has a `Dockerfile`, set it in the config:

```go
buildConfig := builder.Config{
    Name: "my-image",
    Dockerfile: &builder.DockerfileConfig{
        Path: "./Dockerfile",
    },
}
```

### Multi-Architecture Builds

```go
orchestrator := builder.NewBuildOrchestrator(2) // max 2 concurrent

requests := builder.CreateBuildRequests(ctx, &buildConfig)

bldr, _ := buildkit.NewBuildKitBuilder(ctx)
defer bldr.Close()

results, err := orchestrator.BuildMultiArch(ctx, requests, bldr)
```

### Push to Registry

```go
// Push all results to registry.
err = service.Push(ctx, buildConfig, results, opts)

// Or push individually via the builder.
bldr, _ := buildkit.NewBuildKitBuilder(ctx)
digest, err := bldr.Push(ctx, "my-image:latest", "ghcr.io/myorg")
```

## AMI Builds

### Basic AMI Build

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cowdogmoo/warpgate/v3/builder"
    "github.com/cowdogmoo/warpgate/v3/builder/ami"
)

func main() {
    ctx := context.Background()

    clientConfig := ami.ClientConfig{
        Region:  "us-east-1",
        Profile: "default", // or set AccessKeyID/SecretAccessKey
    }

    amiBuilder, err := ami.NewImageBuilderWithAllOptions(ctx, clientConfig, false, ami.MonitorConfig{
        StreamLogs:    true,
        ShowEC2Status: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer amiBuilder.Close()

    buildConfig := builder.Config{
        Name: "my-ami",
        Base: builder.BaseImage{Image: "ami-0abcdef1234567890"},
        Provisioners: []builder.Provisioner{
            {Type: "shell", Inline: []string{"yum update -y"}},
        },
        Targets: []builder.Target{
            {Type: "ami", Region: "us-east-1", InstanceType: "t3.medium"},
        },
    }

    result, err := amiBuilder.Build(ctx, buildConfig)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("AMI: %s in %s (%s)\n", result.AMIID, result.Region, result.Duration)
}
```

### AMI Build with StatusCallback

The `StatusCallback` fires on every poll tick (~30s) with structured progress
data. This replaces the need to parse log output for build stage information.

```go
monitorConfig := ami.MonitorConfig{
    StreamLogs: true,
    StatusCallback: func(update ami.StatusUpdate) {
        if update.StageChanged {
            fmt.Printf("Stage: %s\n", update.StageLabel)
        }
        if update.EstimatedRemaining > 0 {
            fmt.Printf("  Elapsed: %s, ~%s remaining\n",
                update.Elapsed, update.EstimatedRemaining)
        }
    },
}

amiBuilder, _ := ami.NewImageBuilderWithAllOptions(ctx, clientConfig, false, monitorConfig)
```

#### StatusUpdate Fields

| Field                | Type            | Description                     |
| -------------------- | --------------- | ------------------------------- |
| `Stage`              | `string`        | Raw stage name, e.g. `BUILDING` |
| `StageLabel`         | `string`        | Human-readable label            |
| `Elapsed`            | `time.Duration` | Total time since build start    |
| `EstimatedRemaining` | `time.Duration` | Estimated time to completion    |
| `StageChanged`       | `bool`          | `true` on first tick of stage   |

Valid stages: `PENDING`, `CREATING`, `BUILDING`, `TESTING`,
`DISTRIBUTING`, `INTEGRATING`, `AVAILABLE`, `FAILED`.

The callback runs on the polling goroutine. Return quickly to avoid blocking
the poll loop. If you need async processing, send to a channel:

```go
updates := make(chan ami.StatusUpdate, 10)

monitorConfig := ami.MonitorConfig{
    StatusCallback: func(u ami.StatusUpdate) {
        select {
        case updates <- u:
        default: // drop if consumer is slow
        }
    },
}

// Consume in another goroutine.
go func() {
    for u := range updates {
        // update your UI
    }
}()
```

## Progress Display

The `progress` package provides a reusable multi-line progress bar that works
for any concurrent operation. It renders in-place on TTY terminals and falls
back to line-per-change output in CI/log environments.

### Single-Threaded Usage

```go
import "github.com/cowdogmoo/warpgate/v3/progress"

display := progress.NewDisplay(os.Stderr)
bar := display.AddBar("my-task", 1, 1)

bar.Update("Downloading", 0.25, 10*time.Second, 30*time.Second)
display.Render()

bar.Update("Downloading", 0.75, 30*time.Second, 10*time.Second)
display.Render()

bar.Complete()
display.Render()
```

### Concurrent Builds with Progress

When multiple goroutines update bars, use `Start`/`Stop` to run a background
render loop. Callbacks only update bar state -- rendering happens on a single
timer so all bars redraw together as a clean block:

```go
display := progress.NewDisplay(os.Stderr)
display.Start(500 * time.Millisecond)

bar1 := display.AddBar("goad-dc-base", 1, 4)
bar2 := display.AddBar("goad-dc-base-2016", 2, 4)
bar3 := display.AddBar("goad-member-base", 3, 4)
bar4 := display.AddBar("goad-mssql-base", 4, 4)

// Each build's StatusCallback only updates its bar -- never calls Render().
for i, bar := range []*progress.Bar{bar1, bar2, bar3, bar4} {
    bar := bar
    configs[i].MonitorConfig.StatusCallback = func(u ami.StatusUpdate) {
        pct := stageWeightedProgress(u.Stage) // your stage-to-percentage mapping
        bar.Update(u.StageLabel, pct, u.Elapsed, u.EstimatedRemaining)
    }
}

// ... launch builds in goroutines ...
// ... wait for all builds to finish ...

display.Stop() // final render
```

Output (updated in-place every 500ms):

```text
[1/4] goad-dc-base           [██████████████░░░░░░░░░░░] Building       18m14s  ~19m0s remaining
[2/4] goad-dc-base-2016      [██████████████░░░░░░░░░░░] Building       18m14s  ~19m0s remaining
[3/4] goad-member-base       [██████████████░░░░░░░░░░░] Building       18m14s  ~19m0s remaining
[4/4] goad-mssql-base        [██████████████░░░░░░░░░░░] Building       18m14s  ~19m0s remaining
```

### Stage-Weighted Progress

AMI builds go through stages with different durations. Map them to progress
weights so the bar moves proportionally:

```go
func stageWeightedProgress(stage string) float64 {
    weights := map[string]float64{
        "PENDING":      0.05,
        "CREATING":     0.15,
        "BUILDING":     0.60,
        "TESTING":      0.70,
        "DISTRIBUTING": 0.90,
        "INTEGRATING":  0.95,
        "AVAILABLE":    1.00,
    }
    if w, ok := weights[stage]; ok {
        return w
    }
    return 0
}
```

### API Reference

#### Display

```go
func NewDisplay(w io.Writer) *Display       // auto-detect TTY
func NewDisplayTTY(w io.Writer, isTTY bool) // explicit TTY control

func (*Display) AddBar(label string, index, total int) *Bar
func (*Display) SetTotal(total int)          // update all bars' total count
func (*Display) Render()                     // manual redraw (single-threaded)
func (*Display) Start(interval time.Duration) // background render loop
func (*Display) Stop()                       // stop loop + final render
```

#### Bar

```go
func (*Bar) Update(stage string, progress float64, elapsed, remaining time.Duration)
func (*Bar) Complete()                       // sets Done, Progress=1.0
func (*Bar) Fail()                           // sets Error, Stage="Failed"
func (*Bar) IsFinished() bool                // true if Done or Error
```

## Complete Example: Parallel AMI Builds with Progress

This example shows how DreadGOAD-style parallel builds with a live progress
display can be implemented using warpgate as a library:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/cowdogmoo/warpgate/v3/builder"
    "github.com/cowdogmoo/warpgate/v3/builder/ami"
    "github.com/cowdogmoo/warpgate/v3/progress"
)

func main() {
    ctx := context.Background()

    builds := []struct {
        name   string
        config builder.Config
    }{
        {name: "goad-dc-base", config: dcBaseConfig()},
        {name: "goad-dc-base-2016", config: dcBase2016Config()},
        {name: "goad-member-base-2016", config: memberBase2016Config()},
        {name: "goad-mssql-base", config: mssqlBaseConfig()},
    }

    // Create a shared progress display.
    display := progress.NewDisplay(os.Stderr)
    display.Start(500 * time.Millisecond)

    var wg sync.WaitGroup
    results := make([]*builder.BuildResult, len(builds))

    for i, b := range builds {
        wg.Add(1)

        bar := display.AddBar(b.name, i+1, len(builds))

        // Wire the StatusCallback to update only the bar state.
        monitorConfig := ami.MonitorConfig{
            StreamLogs: false, // suppress logs -- progress bar is enough
            StatusCallback: func(u ami.StatusUpdate) {
                pct := stageWeightedProgress(u.Stage)
                bar.Update(u.Stage, pct, u.Elapsed, u.EstimatedRemaining)
            },
        }

        go func(idx int, cfg builder.Config, mc ami.MonitorConfig, bar *progress.Bar) {
            defer wg.Done()

            amiBuilder, err := ami.NewImageBuilderWithAllOptions(
                ctx,
                ami.ClientConfig{Region: "us-east-1"},
                false,
                mc,
            )
            if err != nil {
                bar.Fail()
                return
            }
            defer amiBuilder.Close()

            result, err := amiBuilder.Build(ctx, cfg)
            if err != nil {
                bar.Fail()
                return
            }

            bar.Complete()
            results[idx] = result
        }(i, b.config, monitorConfig, bar)
    }

    wg.Wait()
    display.Stop()

    // Print results.
    for i, r := range results {
        if r != nil {
            fmt.Printf("%s: %s (%s)\n", builds[i].name, r.AMIID, r.Duration)
        }
    }
}

func stageWeightedProgress(stage string) float64 {
    weights := map[string]float64{
        "PENDING":      0.05,
        "CREATING":     0.15,
        "BUILDING":     0.60,
        "TESTING":      0.70,
        "DISTRIBUTING": 0.90,
        "INTEGRATING":  0.95,
        "AVAILABLE":    1.00,
    }
    if w, ok := weights[stage]; ok {
        return w
    }
    return 0
}
```

## Built-in Progress Bars

Warpgate automatically renders progress bars for:

- **Container builds** -- each BuildKit vertex (build step) gets a bar showing
  its name and completion state
- **Docker push** -- each layer gets a bar showing byte-level push progress

These are rendered to stderr using the same `progress` package. No
configuration is needed -- they activate automatically during builds and pushes.
