package payment

import (
	"encoding/json"
	"net/http"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/privatix/dappctrl/data"
)

// serverError is a payment server error.
type serverError struct {
	// Code is a status code.
	Code int `json:"code"`
	// Message is a description of the error.
	Message string `json:"message"`
}

var (
	errInvalidPayload = &serverError{
		Code:    http.StatusBadRequest,
		Message: "",
	}
	errNoChannel = &serverError{
		Code:    http.StatusUnauthorized,
		Message: "Channel is not found",
	}
	errUnexpected = &serverError{
		Code:    http.StatusInternalServerError,
		Message: "An unexpected error occurred",
	}
	errChannelClosed = &serverError{
		Code:    http.StatusUnauthorized,
		Message: "Channel is closed",
	}
	errInvalidAmount = &serverError{
		Code:    http.StatusUnauthorized,
		Message: "Invalid balance amount",
	}
	errInvalidSignature = &serverError{
		Code:    http.StatusUnauthorized,
		Message: "Client signature does not match",
	}
)

func (s *Server) findChannelByBlock(w http.ResponseWriter,
	b uint) (*data.Channel, bool) {
	ch := &data.Channel{}
	if err := s.db.FindOneTo(ch, "block", b); err != nil {
		s.replyError(w, errNoChannel)
		return nil, false
	}
	return ch, true
}

func (s *Server) validateChannelState(w http.ResponseWriter,
	ch *data.Channel) bool {
	if ch.ChannelStatus != data.ChannelActive {
		s.replyError(w, errChannelClosed)
		return false
	}
	return true
}

func (s *Server) validateAmount(w http.ResponseWriter,
	ch *data.Channel, pld *payload) bool {
	if pld.Balance <= ch.ReceiptBalance || pld.Balance > ch.TotalDeposit {
		s.replyError(w, errInvalidAmount)
		return false
	}
	return true
}

func (s *Server) verifySignature(w http.ResponseWriter,
	ch *data.Channel, pld *payload) bool {

	client := &data.User{}
	if s.db.FindOneTo(client, "eth_addr", ch.Client) != nil {
		s.replyError(w, errUnexpected)
		return false
	}

	pub, err := data.ToBytes(client.PublicKey)
	if err != nil {
		s.replyError(w, errUnexpected)
		return false
	}

	sig, err := data.ToBytes(pld.BalanceMsgSig)
	if err != nil {
		s.replyError(w, errUnexpected)
		return false
	}

	if !crypto.VerifySignature(pub, hash(pld), sig[:len(sig)-1]) {
		s.replyError(w, errInvalidSignature)
		return false
	}
	return true
}

func (s *Server) validateChannelForPayment(w http.ResponseWriter,
	ch *data.Channel, pld *payload) bool {
	return s.validateChannelState(w, ch) &&
		s.validateAmount(w, ch, pld) &&
		s.verifySignature(w, ch, pld)
}

func (s *Server) updateChannelWithPayment(w http.ResponseWriter,
	ch *data.Channel, pld *payload) bool {
	ch.ReceiptBalance = pld.Balance
	ch.ReceiptSignature = pld.BalanceMsgSig
	if err := s.db.Update(ch); err != nil {
		s.logger.Warn("failed to update channel: %v", err)
		s.replyError(w, errUnexpected)
		return false
	}
	return true
}

func (s *Server) parsePayload(w http.ResponseWriter,
	r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		s.logger.Warn("failed to parse request body: %v", err)
		s.replyError(w, errInvalidPayload)
		return false
	}
	return true
}

// replyError writes error to reponse.
func (s *Server) replyError(w http.ResponseWriter, reply *serverError) {
	w.WriteHeader(reply.Code)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(reply); err != nil {
		s.logger.Warn("failed to marshal error reply to json: %v", err)
	}
}
