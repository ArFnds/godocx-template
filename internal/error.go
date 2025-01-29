package internal

import "fmt"

type InvalidCommandError struct {
	Message string
	Command string
}

// Implémenter l'interface error pour InvalidCommandError
func (e *InvalidCommandError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Command)
}
func NewInvalidCommandError(message, command string) *InvalidCommandError {
	return &InvalidCommandError{
		Message: message,
		Command: command,
	}
}
