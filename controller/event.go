package controller

import (
	"sync"
)

var mu sync.Mutex

type Event struct {
	EventName string
	Host      string
	Port      string
}
