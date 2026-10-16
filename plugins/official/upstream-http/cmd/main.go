package main

import (
	"encoding/json"
	"fmt"
	httpupstream "github.com/huey1in/moduapi/plugins/official/upstream-http"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
	"os"
)

func main() {
	var network []string
	_ = json.Unmarshal([]byte(os.Getenv("MODUAPI_NETWORK_PERMISSIONS")), &network)
	u := httpupstream.New(network)
	e := plugin.Run(plugin.Definition{Info: &sdk.Description{Id: "upstream.http", Version: "0.1.0", Provides: []string{sdk.KindUpstream}, Capabilities: []string{"streaming"}}, Upstream: u, Close: u.Close})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
