package main

import (
	"fmt"
	"os"

	openai "github.com/huey1in/moduapi/plugins/official/protocol-openai"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
)

func main() {
	err := plugin.Run(plugin.Definition{Info: &sdk.Description{Id: "protocol.openai", Version: "0.1.0", Provides: []string{sdk.KindProtocol}, Capabilities: []string{"streaming", "usage", "affinity"}, Endpoints: []*sdk.Endpoint{{Method: "POST", Path: "/v1/chat/completions", Protocol: "openai.chat"}}}, Protocol: openai.Protocol{}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
