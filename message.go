package main

import (
	"html/template"
)

type MessageType string

const (
	MessageTypeSuccess MessageType = "success"
	MessageTypeInfo    MessageType = "info"
	MessageTypeWarning MessageType = "warning"
	MessageTypeError   MessageType = "error"
)

type Message struct {
	Type    MessageType
	Content template.HTML
}
