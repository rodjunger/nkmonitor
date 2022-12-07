The nkmonitor package provides a monitor for the brazillian swoosh website product restocks.

## Installation

`go get -u github.com/rodjunger/nkmonitor`


## Pre-built binaries

Pre-built binaries for the CLI are available in the releases section.

## CLI usage

`./nkmonitor -u "product url"`

use `./nkmonitor -h` for more details.

## Lib usage 

Errors are intentionally ignored for readability, check cmd/main.go for a more detailed usage example

```go
package main 

import (
    "github.com/rodjunger/nkmonitor"
    "github.com/saucesteals/mimic"
    "time"
)

func main(){
    userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36"

    m, _ := mimic.Chromium(mimic.BrandChrome, "107.0.0.0")
    monitor, _ := nkmonitor.NewMonitor(userAgent, 10 * time.Second, []string{}, m)

    monitor.Start()

    restockCh := make(chan nkmonitor.RestockInfo)
    monitor.Add("https://www.redacted.com.br/snkrs/women's-air-jordan-5-024414.html", restockCh)
    <-restockCh
}
```