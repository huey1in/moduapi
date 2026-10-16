package main

import (
	"fmt"
	single "github.com/huey1in/moduapi/plugins/official/scheduler-single"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
	"os"
)

func main() {
	if e := plugin.Run(plugin.Definition{Info: &sdk.Description{Id: "scheduler.single", Version: "0.1.0", Provides: []string{sdk.KindScheduler}}, Scheduler: single.Scheduler{}}); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
