package alertposttest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/influxdata/kapacitor/alert"
)

type Server struct {
	ts     *httptest.Server
	URL    string
	data   []Request
	closed bool
}

func NewServer(headers map[string]string) *Server {
	s := new(Server)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := Request{MatchingHeaders: true}
		for k, v := range headers {
			nv := r.Header.Get(k)
			if nv != v {
				req.MatchingHeaders = false
			}
		}
		req.Data = alert.AlertData{}
		dec := json.NewDecoder(r.Body)
		dec.Decode(&req.Data)
		s.data = append(s.data, req)
	}))
	s.ts = ts
	s.URL = ts.URL
	return s
}

type Request struct {
	MatchingHeaders bool
	Data            alert.AlertData
}

func (s *Server) Data() []Request {
	return s.data
}

func (s *Server) Close() {
	if s.closed {
		return
	}
	s.closed = true
	s.ts.Close()
}
