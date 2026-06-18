package core

import "fmt"

// Signature for handler lookup error
type HandlerNotFound struct {
	command string
}

func (r *HandlerNotFound) Error() string {
	return fmt.Sprintf("command handler not found: %s", r.command)
}

type DownstreamClientQuitting struct{}

func (r *DownstreamClientQuitting) Error() string {
	return "downstream client qutting"
}
