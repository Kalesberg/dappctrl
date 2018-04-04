package uisrv

import (
	"net/http"

	"github.com/privatix/dappctrl/data"
)

// handleChannels calls appropriate handler by scanning incoming request.
func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	if id := idFromStatusPath(channelsPath, r.URL.Path); id != "" {
		if r.Method == "GET" {
			s.handleGetChannelStatus(w, r, id)
			return
		}
		if r.Method == "PUT" {
			s.handlePutChannelStatus(w, r, id)
			return
		}
	} else {
		if r.Method == "GET" {
			s.handleGetChannels(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// handleGetChannels replies with all channels or a channel by id.
func (s *Server) handleGetChannels(w http.ResponseWriter, r *http.Request) {
	s.handleGetResources(w, r, &getConf{
		Params: []queryParam{{Name: "id", Field: "id"}},
		View:   data.ChannelTable,
	})
}

// handleGetChannelStatus replies with channels status by id.
func (s *Server) handleGetChannelStatus(w http.ResponseWriter, r *http.Request, id string) {
	channel := &data.Channel{}
	if !s.findByID(w, channel, id) {
		return
	}
	s.replyStatus(w, channel.ChannelStatus)
}

func (s *Server) handlePutChannelStatus(w http.ResponseWriter, r *http.Request, id string) {
	// TODO once job queue implemented.
}