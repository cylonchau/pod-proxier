package haproxy

import (
	"github.com/haproxytech/models/v2"
)

type HaproxyInterface interface {
	EnsureHaproxy() bool
	// EnsureFrontendBind checks if address is bound to the frontend and, if not, binds it.  If the frontend is already bound, return true.
	EnsureFrontendBind(bindName, frontendName string) bool
	// EnsureBackend checks if backend is already created return false.
	EnsureBackend(name string) bool
	// EnsureFrontend checks if frontend is already created return false.
	EnsureFrontend(name string) bool
	// UnbindFrontend unbind address from the frontend
	unbindFrontend(name string) (exist bool, err error)
	// DeleteBackend deletes the given Backend by name.
	DeleteBackend(name string, txID string) bool
	// DeleteFrontend deletes the given frontend by name.
	DeleteFrontend(name string, txID string) bool

	DeleteServerFromBackend(serverName, backendName string, txID string) (bool, error)

	checkPortIsAvailable(protocol string, port int) (status bool)

	getVersion() int

	GetServer(backendName, srvName string) Server

	GetServers(backendName string) Servers

	AddFrontend(payload *models.Frontend, txID string) (bool, error)
	AddBind(payload *models.Bind, frontendName string, txID string) (bool, error)
	AddBackend(payload *models.Backend, txID string) (bool, error)
	AddServerToBackend(payload *Server, backendName string, txID string) (bool, error)

	StartTransaction() (string, error)
	CommitTransaction(id string) error
	DiscardTransaction(id string) error
}
