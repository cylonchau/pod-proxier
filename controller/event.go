package controller

import (
	"sync"
)

var mu sync.Mutex

type Event struct {
	EventName string
	Host      string
	Port      int
}

var serviceMu sync.Mutex

type ServiceCleanupEvent struct {
	ServiceKey string // format: "namespace/serviceName"
}

// ServiceInfo represents service information for API responses
type ServiceInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Ports     []ServicePortInfo `json:"ports"`
	Selector  map[string]string `json:"selector"`
	Type      string            `json:"type"`
}

type ServicePortInfo struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"target_port"`
	Protocol   string `json:"protocol"`
}

type ServiceMappingList struct {
	Total    int              `json:"total"`
	Mappings []ServiceMapping `json:"mappings"`
}
