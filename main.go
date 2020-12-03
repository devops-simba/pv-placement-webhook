package main

import (
	webhookCore "github.com/devops-simba/webhook_core"
	log "github.com/golang/glog"
)

func main() {
	command := webhookCore.ReadCommand(
		"",
		nil,
		false,
		NewPvPlacementModifier(),
		NewPvPlacementValidator(),
	)

	if err := command.Execute(); err != nil {
		log.Fatalf("Failed to execute the command: %v", err)
	}
}
