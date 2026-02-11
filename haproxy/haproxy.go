package haproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	restclient "github.com/cylonchau/gorest"
	models "github.com/haproxytech/models/v2"
	json "github.com/json-iterator/go"
	"github.com/shirou/gopsutil/v3/process"
	"k8s.io/klog/v2"
)

var _ HaproxyInterface = &HaproxyHandle{}

type HaproxyHandle struct {
	localAddr string
	host      string
	user      string
	password  string
	mu        sync.Mutex
}

func (h *HaproxyHandle) EnsureHaproxy() bool {
	return true
}

func (h *HaproxyHandle) newRequest() *restclient.Request {
	req := restclient.NewDefaultRequest().BasicAuth(h.user, h.password).Host(h.host)
	req.AddHeader("Content-Type", "application/json")
	return req
}

func (h *HaproxyHandle) EnsureFrontendBind(bindName, frontendName string) bool {
	// parent_type is frontend fix format
	// parent_name is frontend name
	// return all bind fields of frontend
	url := fmt.Sprintf("%s/%s?parent_type=frontend&parent_name=%s", BIND, url.QueryEscape(bindName), url.QueryEscape(frontendName))

	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	if resp.Err != nil {
		klog.Error(resp.Err)
		return false
	}
	bindList := map[string][]models.Bind{}
	json.Unmarshal(resp.Body, &bindList)
	if len(bindList["data"]) > 0 {
		klog.Errorf("frontend %s has been exist bind [%s].", bindName, frontendName)
		return false
	}
	return true
}

// ensure
func (h *HaproxyHandle) EnsureBackend(backendName string) bool {
	url := fmt.Sprintf("%s/%s", BACKEND, url.QueryEscape(backendName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	klog.V(4).Infof("Opeate query backend %s", backendName)
	exist, _ := handleError(&resp, backendName)
	return exist
}

func (h *HaproxyHandle) EnsureFrontend(frontendName string) bool {
	url := fmt.Sprintf("%s/%s", FRONTEND, url.QueryEscape(frontendName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	klog.V(4).Infof("Opeate query frontend %s", frontendName)
	exist, _ := handleError(&resp, frontendName)
	return exist
}

func (h *HaproxyHandle) EnsureServer(serverName, backendName string) bool {
	v := h.getVersion()
	url := fmt.Sprintf("%s/%s?parent_type=backend&parent_name=%s&version=%d", SERVER, url.QueryEscape(serverName), url.QueryEscape(backendName), v)
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	klog.V(4).Infof("Opeate query server %s", serverName)
	exist, _ := handleError(&resp, backendName+":"+serverName)
	return exist
}

func (h *HaproxyHandle) ensureOneBind(bindName, frontendName string) (exist bool, err error) {
	v := h.getVersion()
	url := fmt.Sprintf("%s/%s?parent_type=frontend&frontend=%s&version=%d", BIND, url.QueryEscape(bindName), url.QueryEscape(frontendName), v)
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	klog.V(4).Infof("Opeate get one bind %s", bindName)
	return handleError(&resp, bindName)
}

// getVersionOrDefault returns transaction_id=txID if txID is not empty, otherwise version=v
func (h *HaproxyHandle) getVersionOrTxID(txID string) string {
	if txID != "" {
		return fmt.Sprintf("transaction_id=%s", txID)
	}
	return fmt.Sprintf("version=%d", h.getVersion())
}

// backend series
func (h *HaproxyHandle) AddBackend(payload *models.Backend, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s?%s", BACKEND, h.getVersionOrTxID(txID))
	body, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Failed to json convert Backend marshal: %s\n", err)
		return false, err
	}

	resp := h.newRequest().Path(url).Post().Body(body).Do(context.TODO())

	return handleError(&resp, payload)
}

func (h *HaproxyHandle) GetBackend(backendName string) models.Backend {
	url := fmt.Sprintf("%s/%s", BACKEND, url.QueryEscape(backendName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	if resp.Err != nil {
		return models.Backend{}
	}
	var (
		rawJson       map[string]json.RawMessage
		backendobj    models.Backend
		incurredError error
	)

	if incurredError = json.Unmarshal(resp.Body, &rawJson); incurredError == nil {
		if incurredError = json.Unmarshal(rawJson["data"], &backendobj); incurredError == nil {
			return backendobj
		}
	}
	klog.Errorf("Failed to json convert model.Backend: %s", incurredError)
	return models.Backend{}
}

func (h *HaproxyHandle) GetBackends() models.Backends {
	url := fmt.Sprintf("%s", BACKEND)
	resp := h.newRequest().Path(url).Get().Do(context.TODO())

	if resp.Err != nil {
		klog.Errorf("Failed to request: %s\n", resp.Err)
		return models.Backends{}
	}

	var (
		rawJson       map[string]json.RawMessage
		backends      models.Backends
		incurredError error
	)
	incurredError = json.Unmarshal(resp.Body, &rawJson)
	if incurredError == nil {
		incurredError = json.Unmarshal(rawJson["data"], &backends)
		if incurredError == nil {
			return backends
		}
	}
	klog.Errorf("Failed to json convert model.Backends: %s", incurredError)
	return models.Backends{}
}

func (h *HaproxyHandle) GetServices() Services {
	backends := h.GetBackends()
	frontends := h.GetFrontends()
	if len(frontends) == 0 || len(backends) == 0 {
		return nil
	}
	var bs = make(map[string]models.Backend)
	for _, item := range backends {
		bs[item.Name] = *item
	}

	var services Services
	for _, item := range frontends {
		if item.Name == "stats" {
			continue
		}
		var service Service
		service.Name = strings.Trim(item.Name, "FRONTEND.")
		service.Frontend = *item
		service.Backend = bs[service.Frontend.DefaultBackend]
		services = append(services, service)
	}
	return services
}

func (h *HaproxyHandle) DeleteBackend(backendName string, txID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	klog.V(3).Infof("Opeate delete backend [%s]", backendName)
	url := fmt.Sprintf("%s/%s?%s", BACKEND, url.QueryEscape(backendName), h.getVersionOrTxID(txID))
	resp := h.newRequest().Path(url).Delete().Do(context.TODO())
	b, _ := handleError(&resp, backendName)
	return b
}

func (h *HaproxyHandle) ReplaceBackend(oldName string, new *models.Backend, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?%s", BACKEND, url.QueryEscape(oldName), h.getVersionOrTxID(txID))
	body, err := json.Marshal(new)
	if err != nil {
		klog.Errorf("Failed to json convert Backend marshal: %s\n", body)
		return false, err
	}
	resp := h.newRequest().Path(url).Put().Body(body).Do(context.TODO())
	klog.V(4).Infof("Opeate replace backend [%s] to [%s]", oldName, new.Name)
	return handleError(&resp, new)
}

// frontend series
func (h *HaproxyHandle) DeleteFrontend(frontendName string, txID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?%s", FRONTEND, url.QueryEscape(frontendName), h.getVersionOrTxID(txID))
	resp := h.newRequest().Path(url).Delete().Do(context.TODO())
	klog.V(3).Infof("Opeate delete frontend [%s]", frontendName)
	b, _ := handleError(&resp, frontendName)
	return b
}

func (h *HaproxyHandle) AddFrontend(payload *models.Frontend, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s?%s", FRONTEND, h.getVersionOrTxID(txID))
	body, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Failed to json convert Frontend marshal: %s\n", err)
		return false, err
	}
	resp := h.newRequest().Path(url).Body(body).Post().Do(context.TODO())
	klog.V(4).Infof("Opeate add a frontend [%s]", payload.Name)
	return handleError(&resp, payload)
}

func (h *HaproxyHandle) GetFrontend(frontendName string) models.Frontend {
	url := fmt.Sprintf("%s/%s", FRONTEND, url.QueryEscape(frontendName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	if resp.Err != nil {
		return models.Frontend{}
	}

	var (
		rawJson       map[string]json.RawMessage
		frontend      models.Frontend
		incurredError error
	)

	if incurredError = json.Unmarshal(resp.Body, &rawJson); incurredError == nil {
		incurredError = json.Unmarshal(rawJson["data"], &frontend)
		if incurredError == nil {
			return frontend
		}
	}

	klog.Errorf("Failed to json convert models.Frontend marshal: %s", incurredError)
	return models.Frontend{}
}

func (h *HaproxyHandle) GetFrontends() models.Frontends {
	url := fmt.Sprintf("%s", FRONTEND)
	resp := h.newRequest().Path(url).Get().Do(context.TODO())

	if resp.Err != nil {
		klog.Errorf("Failed to request: %s\n", resp.Err)
		return models.Frontends{}
	}
	var (
		rawJson       map[string]json.RawMessage
		frontends     models.Frontends
		incurredError error
	)

	incurredError = json.Unmarshal(resp.Body, &rawJson)
	if incurredError == nil {
		incurredError = json.Unmarshal(rawJson["data"], &frontends)
		if incurredError == nil {
			return frontends
		}
	}
	klog.Errorf("Failed to json convert models.Frontends marshal: %s", incurredError)
	return models.Frontends{}
}

func (h *HaproxyHandle) ReplaceFrontend(oldName string, new *models.Frontend, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?%s", FRONTEND, url.QueryEscape(oldName), h.getVersionOrTxID(txID))
	body, err := json.Marshal(new)
	if err != nil {
		klog.Errorf("Failed to json convert Frontend marshal: %s\n", err)
		return false, err
	}
	resp := h.newRequest().Path(url).Body(body).Put().Do(context.TODO())
	klog.V(4).Infof("Opeate replace frontend [%s] to [%s]", oldName, new.Name)
	return handleError(&resp, new)
}

// haproxy server operations
func (h *HaproxyHandle) AddServerToBackend(payload *Server, backendName string, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s?parent_type=backend&parent_name=%s&%s", SERVER, url.QueryEscape(backendName), h.getVersionOrTxID(txID))
	body, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Failed to json convert Models.Server: %s\n", body)
		return false, err
	}

	resp := h.newRequest().Path(url).Body(body).Post().Do(context.TODO())
	return handleError(&resp, payload)
}

func (h *HaproxyHandle) DeleteServerFromBackend(serverName, backendName string, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?parent_type=backend&parent_name=%s&%s", SERVER, url.QueryEscape(serverName), url.QueryEscape(backendName), h.getVersionOrTxID(txID))
	resp := h.newRequest().Path(url).Delete().Do(context.TODO())

	return handleError(&resp, serverName)
}

func (h *HaproxyHandle) ReplaceServerFromBackend(oldName string, new *Server, backendName string, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?parent_type=backend&parent_name=%s&%s", SERVER, url.QueryEscape(oldName), url.QueryEscape(backendName), h.getVersionOrTxID(txID))
	body, err := json.Marshal(new)
	if err != nil {
		klog.Errorf("Failed to json convert Backend: %s\n", body)
		return false, err
	}
	resp := h.newRequest().Path(url).Put().Body(body).Do(context.TODO())
	klog.V(4).Infof("Opeate replace server [%s] to [%s_%s%d]", oldName, new.Name, new.Address, new.Port)
	return handleError(&resp, new)
}

func (h *HaproxyHandle) GetServers(backendName string) Servers {
	url := fmt.Sprintf("%s?parent_type=backend&parent_name=%s", SERVER, url.QueryEscape(backendName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	if resp.Err != nil {
		klog.Errorf("Failed to request: %s\n", resp.Err)
		return Servers{}
	}

	var (
		rawJson       map[string]json.RawMessage
		servers       Servers
		incurredError error
	)

	incurredError = json.Unmarshal(resp.Body, &rawJson)
	if incurredError == nil {
		incurredError = json.Unmarshal(rawJson["data"], &servers)
		if incurredError == nil {
			return servers
		}
	}
	klog.Errorf("Failed to json convert Servers marshal: %s", incurredError)
	return servers
}

func (h *HaproxyHandle) GetServer(backendName, srvName string) Server {
	url := fmt.Sprintf("%s/%s?parent_type=backend&parent_name=%s", SERVER, srvName, url.QueryEscape(backendName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	if resp.Err != nil {
		klog.Errorf("Failed to request: %s\n", resp.Err)
		return Server{}
	}
	var (
		rawJson       map[string]json.RawMessage
		server        Server
		incurredError error
	)

	if incurredError = json.Unmarshal(resp.Body, &rawJson); incurredError == nil {
		if incurredError = json.Unmarshal(rawJson["data"], &server); incurredError == nil {
			return server
		}
	}
	klog.Errorf("Failed to json convert Server marshal: %s", incurredError)
	return Server{}
}

// bind
func (h *HaproxyHandle) unbindFrontend(bindName string) (exist bool, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v := h.getVersion()
	url := fmt.Sprintf("%s?parent_type=frontend&parent_name=%s&version=%d", BIND, url.QueryEscape(bindName), v)
	resp := h.newRequest().Path(url).Delete().Do(context.TODO())
	klog.V(4).Infof("Opeate unbind frontend %s", bindName)
	return handleError(&resp, bindName)
}

func (h *HaproxyHandle) GetBind(bindName, frontName string) models.Bind {
	url := fmt.Sprintf("%s/%s?parent_type=frontend&frontend=%s", BIND, url.QueryEscape(bindName), url.QueryEscape(frontName))
	resp := h.newRequest().Path(url).Get().Do(context.TODO())
	if resp.Err != nil {
		klog.V(4).Info(resp.Err)
		return models.Bind{}
	}

	var (
		rawJson       map[string]json.RawMessage
		bind          models.Bind
		incurredError error
	)

	if incurredError = json.Unmarshal(resp.Body, &rawJson); incurredError == nil {
		if incurredError = json.Unmarshal(rawJson["data"], &bind); incurredError == nil {
			return bind
		}
	}
	klog.Errorf("Failed to json convert models.Bind marshal: %s", incurredError)
	return models.Bind{}
}

func (h *HaproxyHandle) AddBind(payload *models.Bind, frontendName string, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s?parent_type=frontend&frontend=%s&%s", BIND, url.QueryEscape(frontendName), h.getVersionOrTxID(txID))
	body, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Failed to json convert Modes.bind marshal: %s\n", err)
		return false, err
	}
	resp := h.newRequest().Path(url).Post().Body(body).Do(context.TODO())

	return handleError(&resp, payload)
}

func (h *HaproxyHandle) DeleteBind(bindName, frontendName string, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?parent_type=frontend&frontend=%s&%s", BIND, url.QueryEscape(bindName), url.QueryEscape(frontendName), h.getVersionOrTxID(txID))
	resp := h.newRequest().Path(url).Delete().Do(context.TODO())
	return handleError(&resp, bindName)
}

func (h *HaproxyHandle) replaceBind(oldName, frontendName string, new *models.Bind, txID string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	url := fmt.Sprintf("%s/%s?parent_type=frontend&frontend=%s&%s", BIND, url.QueryEscape(oldName), url.QueryEscape(frontendName), h.getVersionOrTxID(txID))
	body, err := json.Marshal(new)
	if err != nil {
		klog.Errorf("Failed to json convert Models.Bind: %s", body)
		return false, err
	}
	klog.V(4).Infof("Opeate replace bind [%s] to [%s]", oldName, new.Name)
	resp := h.newRequest().Path(url).Put().Body(body).Do(context.TODO())
	return handleError(&resp, new)
}

// common series
func (h *HaproxyHandle) checkPortIsAvailable(protocol string, port int) (status bool) {

	conn, err := net.DialTimeout(protocol, net.JoinHostPort(h.localAddr, strconv.Itoa(port)), time.Duration(checkTimeout)*time.Second)
	if err != nil {
		opErr, ok := err.(*net.OpError)
		if ok && strings.Contains(opErr.Err.Error(), "refused") {
			status = true
			return
		} else if opErr.Timeout() {
			return
		} else {
			return
		}
	}

	if conn != nil {
		defer conn.Close()
		return
	}
	return
}

func (h *HaproxyHandle) getVersion() int {
	resp := h.newRequest().Path(VERSION).Get().Do(context.TODO())
	if resp.Err != nil {
		return -1
	}
	version, _ := strconv.Atoi(strings.TrimSpace(string(resp.Body)))
	return version
}

func (h *HaproxyHandle) StartTransaction() (string, error) {
	v := h.getVersion()
	url := fmt.Sprintf("%s?version=%d", TRANSACTION, v)
	resp := h.newRequest().Path(url).Post().Do(context.TODO())
	if resp.Err != nil {
		return "", resp.Err
	}
	if resp.Code != http.StatusCreated {
		return "", fmt.Errorf("failed to start transaction: %s", string(resp.Body))
	}
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		return "", err
	}
	return data["id"].(string), nil
}

func (h *HaproxyHandle) CommitTransaction(id string) error {
	url := fmt.Sprintf("%s/%s", TRANSACTION, id)
	resp := h.newRequest().Path(url).Put().Do(context.TODO())
	if resp.Err != nil {
		return resp.Err
	}
	if resp.Code != http.StatusAccepted && resp.Code != http.StatusOK {
		return fmt.Errorf("failed to commit transaction: %s", string(resp.Body))
	}
	return nil
}

func (h *HaproxyHandle) DiscardTransaction(id string) error {
	url := fmt.Sprintf("%s/%s", TRANSACTION, id)
	resp := h.newRequest().Path(url).Delete().Do(context.TODO())
	if resp.Err != nil {
		return resp.Err
	}
	if resp.Code != http.StatusNoContent && resp.Code != http.StatusNotFound {
		return fmt.Errorf("failed to discard transaction: %s", string(resp.Body))
	}
	return nil
}

// useful links
// https://stackoverflow.com/questions/27410764/dial-with-a-specific-address-interface-golang
// https://stackoverflow.com/questions/22751035/golang-distinguish-ipv4-ipv6
func GetLocalAddr(dev string) (addr net.IP, err error) {
	var (
		ief      *net.Interface
		addrs    []net.Addr
		ipv4Addr net.IP
	)
	if ief, err = net.InterfaceByName(dev); err != nil { // get interface
		return
	}
	if addrs, err = ief.Addrs(); err != nil { // get addresses
		return
	}
	for _, addr := range addrs { // get ipv4 address
		if ipv4Addr = addr.(*net.IPNet).IP.To4(); ipv4Addr != nil {
			break
		}
	}
	if ipv4Addr == nil {
		return net.IP{}, errors.New(fmt.Sprintf("interface %s don't have an ipv4 address", dev))
	}
	return ipv4Addr, nil
}

func handleError(response *restclient.Response, res interface{}) (bool, error) {
	var log200, log201, log202, log204, log400, log409, log404 string
	switch res.(type) {
	case string:
		log200 = fmt.Sprintf("The resource %s Successful operation.", res)
		log202 = fmt.Sprintf("The resource %s Configuration change accepted.", res)
		log204 = fmt.Sprintf("The resource %s deleted.", res)
		log404 = fmt.Sprintf("The specified resource %s was not found", res)
	case models.Bind:
		var payload = res.(models.Bind)
		log200 = fmt.Sprintf("The resource %s Successful operation.", payload.Name)
		log202 = fmt.Sprintf("The resource %s Configuration change accepted.", payload.Name)
		log201 = fmt.Sprintf("The resource %s created", payload.Name)
		log400 = fmt.Sprintf("Bad request bind: %s", "bind", payload.Name)
		log404 = fmt.Sprintf("The specified resource %s was not found ", payload.Name)
		log409 = fmt.Sprintf("The specified resource %s already exists.", payload.Name)
	case models.Frontend:
		var payload = res.(models.Frontend)
		log200 = fmt.Sprintf("The resource %s Successful operation.", payload.Name)
		log202 = fmt.Sprintf("The resource %s Configuration change accepted.", payload.Name)
		log201 = fmt.Sprintf("The resource %s created", payload.Name)
		log400 = fmt.Sprintf("Bad request frontend: %s", payload.Name)
		log404 = fmt.Sprintf("The specified resource was not found %s", payload.Name)
		log409 = fmt.Sprintf("The specified resource %s already exists.", payload.Name)
	case models.Backend:
		var payload = res.(models.Backend)
		log200 = fmt.Sprintf("The resource %s Successful operation.", payload.Name)
		log202 = fmt.Sprintf("The resource %s Configuration change accepted.", payload.Name)
		log201 = fmt.Sprintf("The resource %s created", payload.Name)
		log400 = fmt.Sprintf("Bad request backend: %s", payload.Name)
		log404 = fmt.Sprintf("The specified resource was not found %s", payload.Name)
		log409 = fmt.Sprintf("The specified resource %s already exists.", payload.Name)
	case Server:
		var payload = res.(Server)
		log202 = fmt.Sprintf("The resource %s Successful operation.", payload.Name)
		log200 = fmt.Sprintf("The resource %s Configuration change accepted.", payload.Name)
		log201 = fmt.Sprintf("The resource %s created", payload.Name)
		log400 = fmt.Sprintf("Bad request server: %s", payload.Name)
		log404 = fmt.Sprintf("The specified resource was not found %s", payload.Name)
		log409 = fmt.Sprintf("The specified resource %s already exists.", payload.Name)
	}
	if response.Err == nil {
		switch response.Code {
		case 0:
			klog.V(4).Info(log200)
			return true, nil
		case http.StatusOK:
			klog.V(4).Info(log200)
			return true, nil
		case http.StatusCreated:
			klog.V(4).Info(log201)
			return true, nil
		case http.StatusAccepted:
			klog.V(4).Info(log202)
			return true, nil
		case http.StatusNoContent:
			klog.V(4).Info(log204)
			return true, fmt.Errorf(log204)
		case http.StatusBadRequest:
			klog.V(4).Info(log400)
			return false, fmt.Errorf(log400)
		case http.StatusNotFound:
			klog.V(4).Info(log404)
			return false, fmt.Errorf(log404)
		case http.StatusConflict:
			klog.V(4).Info(log409)
			return true, fmt.Errorf(log409)
		default:
			klog.V(4).Info("Unkown error, %s", string(response.Body))
			return false, fmt.Errorf("Unkown error, %s", string(response.Body))
		}
	}
	return false, response.Err
}

func getProcessByName(processName string) bool {
	processes, err := process.Processes()
	if err != nil {
		return false
	}
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			return false
		}
		if strings.Contains(name, processName) {
			return true
		}
	}
	return false
}

func NewHaproxyHandle(user, password, host string) HaproxyHandle {
	var addr string

	if addr == "" {
		addr = "127.0.0.1:5555"
	}
	return HaproxyHandle{
		localAddr: addr,
		host:      host,
		user:      user,
		password:  password,
	}
}
