package http

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/boreq/errors"
	"github.com/boreq/rest"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/planetary-social/nos-event-service/internal/logging"
	prometheusadapters "github.com/planetary-social/nos-event-service/service/adapters/prometheus"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays/transport"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	config     config.Config
	logger     logging.Logger
	app        app.Application
	prometheus *prometheusadapters.Prometheus
}

func NewServer(
	config config.Config,
	logger logging.Logger,
	app app.Application,
	prometheus *prometheusadapters.Prometheus,
) Server {
	return Server{
		config:     config,
		logger:     logger.New("server"),
		app:        app,
		prometheus: prometheus,
	}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", s.config.ListenAddress())
	if err != nil {
		return errors.Wrap(err, "error listening")
	}

	go func() {
		<-ctx.Done()
		if err := listener.Close(); err != nil {
			s.logger.Error().WithError(err).Message("error closing listener")
		}
	}()

	mux := s.createMux()
	return http.Serve(listener, mux)
}

func (s *Server) createMux() http.Handler {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.HandlerFor(s.prometheus.Registry(), promhttp.HandlerOpts{}))
	r.HandleFunc("/events/{id}", rest.Wrap(s.serveEvents))
	r.HandleFunc("/public-keys/{hex}", rest.Wrap(s.servePublicKey))
	r.HandleFunc("/", s.serveWs)
	return r
}

func (s *Server) serveWs(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		s.logger.Error().WithError(err).Message("error upgrading the connection")
		return
	}

	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Error().WithError(err).Message("error closing the connection")
		}
	}()

	if err := s.handleConnection(ctx, conn); err != nil {
		closeErr := &websocket.CloseError{}
		if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseNormalClosure {
			s.logger.Error().WithError(err).Message("error handling the connection")
		}
	}
}

func (s *Server) serveEvents(r *http.Request) rest.RestResponse {
	switch r.Method {
	case http.MethodGet:
		vars := mux.Vars(r)
		idString := vars["id"]

		eventID, err := domain.NewEventIdFromHex(idString)
		if err != nil {
			return rest.ErrBadRequest.WithMessage("event id must be in hex")
		}

		event, err := s.app.GetEvent.Handle(r.Context(), app.NewGetEvent(eventID))
		if err != nil {
			if errors.Is(err, app.ErrEventNotFound) {
				return rest.ErrNotFound
			}

			s.logger.Error().WithError(err).Message("error getting an event")
			return rest.ErrInternalServerError
		}

		return rest.NewResponse(newGetEventResponse(event))
	default:
		return rest.ErrMethodNotAllowed
	}
}

type getEventResponse struct {
	Event json.RawMessage `json:"event"`
}

func newGetEventResponse(event domain.Event) getEventResponse {
	return getEventResponse{Event: event.Raw()}
}

func (s *Server) servePublicKey(r *http.Request) rest.RestResponse {
	vars := mux.Vars(r)
	hexPublicKeyString := vars["hex"]

	publicKey, err := domain.NewPublicKeyFromHex(hexPublicKeyString)
	if err != nil {
		return rest.ErrBadRequest.WithMessage("invalid hex public key")
	}

	publicKeyInfo, err := s.app.GetPublicKeyInfo.Handle(r.Context(), app.NewGetPublicKeyInfo(publicKey))
	if err != nil {
		s.logger.Error().WithError(err).Message("error getting public key info")
		return rest.ErrInternalServerError
	}

	return rest.NewResponse(newGetPublicKeyResponse(publicKeyInfo))
}

type getPublicKeyResponse struct {
	Followers int `json:"followers"`
	Followees int `json:"followees"`
}

func newGetPublicKeyResponse(info app.PublicKeyInfo) *getPublicKeyResponse {
	return &getPublicKeyResponse{
		Followers: info.NumberOfFollowers(),
		Followees: info.NumberOfFollowees(),
	}
}

func (s *Server) handleConnection(ctx context.Context, conn *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			return errors.Wrap(err, "error reading the websocket message")
		}

		message := nostr.ParseMessage(messageBytes)
		if message == nil {
			s.logger.
				Error().
				WithError(err).
				WithField("message", string(messageBytes)).
				Message("error parsing a message")
			return errors.New("failed to parse a message")
		}

		switch v := message.(type) {
		case *nostr.EventEnvelope:
			msg := s.processEventReturningOK(ctx, v.Event)

			msgJSON, err := msg.MarshalJSON()
			if err != nil {
				return errors.Wrap(err, "error marshaling a message")
			}

			if err := conn.WriteMessage(websocket.TextMessage, msgJSON); err != nil {
				return errors.Wrap(err, "error writing a message")
			}
		default:
			s.logger.Error().WithField("message", message).Message("received an unknown message")
			return errors.New("unknown message received")
		}
	}
}

func (s *Server) processEventReturningOK(ctx context.Context, event nostr.Event) transport.MessageOK {
	if err := s.processEvent(ctx, event); err != nil {
		s.logger.
			Error().
			WithError(err).
			Message("error processing an event")
		return transport.NewMessageOKWithError(event.ID, err.Error())
	}
	return transport.NewMessageOKWithSuccess(event.ID)
}

func (s *Server) processEvent(ctx context.Context, libevent nostr.Event) error {
	event, err := domain.NewEvent(libevent)
	if err != nil {
		return errors.Wrap(err, "error creating an event")
	}

	registration, err := domain.NewRegistrationFromEvent(event)
	if err != nil {
		return errors.Wrap(err, "error creating a registration")
	}

	cmd := app.NewAddPublicKeyToMonitor(registration.PublicKey())

	if err := s.app.AddPublicKeyToMonitor.Handle(ctx, cmd); err != nil {
		return errors.Wrap(err, "error calling the handler")
	}

	return nil
}
