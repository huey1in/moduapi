package main

import (
	"fmt"
	rr "github.com/huey1in/moduapi/plugins/official/scheduler-roundrobin"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
	"os"
)

func main() {
	if e := plugin.Run(plugin.Definition{Info: &sdk.Description{Id: "scheduler.roundrobin", Version: "0.1.0", Provides: []string{sdk.KindScheduler}}, Scheduler: &rr.Scheduler{}}); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
