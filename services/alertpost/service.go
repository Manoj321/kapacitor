package alertpost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/influxdata/kapacitor/alert"
	"github.com/influxdata/kapacitor/bufpool"
)

type Service struct {
	mu        sync.RWMutex
	endpoints map[string]Config
	logger    *log.Logger
}

func NewService(c Configs, l *log.Logger) *Service {
	s := &Service{
		logger:    l,
		endpoints: c.index(),
	}
	return s
}

type HandlerConfig struct {
	URL      string `mapstructure:"url"`
	Endpoint string `mapstructure:"endpoint"`
}

type handler struct {
	s        *Service
	bp       *bufpool.Pool
	url      string
	endpoint string
	logger   *log.Logger
}

func (s *Service) Handler(c HandlerConfig, l *log.Logger) alert.Handler {
	return &handler{
		s:        s,
		bp:       bufpool.New(),
		url:      c.URL,
		endpoint: c.Endpoint,
		logger:   l,
	}
}

func (s *Service) Open() error {
	return nil
}

func (s *Service) Close() error {
	return nil
}

func (s *Service) endpoint(e string) (c Config, ok bool) {
	s.mu.RLock()
	c, ok = s.endpoints[e]
	s.mu.RUnlock()
	return
}

func (s *Service) Update(newConfigs []interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, nc := range newConfigs {
		if c, ok := nc.(Config); ok {
			s.endpoints[c.Endpoint] = c
		} else {
			return fmt.Errorf("unexpected config object type, got %T exp %T", nc, c)
		}
	}
	return nil
}

func (s *Service) Test(options interface{}) error {
	var err error
	o, ok := options.(*testOptions)
	if !ok {
		return fmt.Errorf("unexpected options type %t", options)
	}

	event := alert.Event{}
	body := bytes.NewBuffer(nil)
	ad := alertDataFromEvent(event)

	err = json.NewEncoder(body).Encode(ad)
	if err != nil {
		return fmt.Errorf("failed to marshal alert data json: %v", err)
	}

	// Create the HTTP request
	var req *http.Request
	c := Config{
		Endpoint: o.Endpoint,
		URL:      o.URL,
		Headers:  o.Headers,
	}
	req, err = c.NewRequest(body)

	// Execute the request
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to POST alert data: %v", err)
	}
	resp.Body.Close()
	return nil
}

type testOptions struct {
	Endpoint string            `json:"endpoint"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
}

func (s *Service) TestOptions() interface{} {
	return &testOptions{
		Endpoint: "example",
		URL:      "http://localhost:3000/",
		Headers:  map[string]string{"Auth": "secret"},
	}
}

// Prefers URL over Endpoint
func (h *handler) Handle(event alert.Event) {
	var err error

	// Construct the body of the HTTP request
	body := h.bp.Get()
	defer h.bp.Put(body)
	ad := alertDataFromEvent(event)

	err = json.NewEncoder(body).Encode(ad)
	if err != nil {
		h.logger.Printf("E! failed to marshal alert data json: %v", err)
		return
	}

	// Create the HTTP request
	var req *http.Request
	if h.url != "" {
		req, err = http.NewRequest("POST", h.url, body)
		if err != nil {
			h.logger.Printf("E! failed to create POST request: %v", err)
			return
		}
	} else {
		c, ok := h.s.endpoint(h.endpoint)
		if !ok {
			h.logger.Printf("E! endpoint does not exist: %v", h.endpoint)
			return
		}
		req, err = c.NewRequest(body)
	}

	// Execute the request
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.logger.Printf("E! failed to POST alert data: %v", err)
		return
	}
	resp.Body.Close()
}

func alertDataFromEvent(event alert.Event) alert.AlertData {
	return alert.AlertData{
		ID:       event.State.ID,
		Message:  event.State.Message,
		Details:  event.State.Details,
		Time:     event.State.Time,
		Duration: event.State.Duration,
		Level:    event.State.Level,
		Data:     event.Data.Result,
	}
}
