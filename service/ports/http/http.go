package http

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/boreq/errors"
	"github.com/boreq/rest"
	"github.com/planetary-social/nos-event-service/internal/logging"
	prometheusadapters "github.com/planetary-social/nos-event-service/service/adapters/prometheus"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain"
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

func (s *Server) createMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.prometheus.Registry(), promhttp.HandlerOpts{}))
	mux.Handle("/public-keys-to-monitor", rest.Wrap(s.publicKeysToMonitor))
	return mux
}

func (s *Server) publicKeysToMonitor(r *http.Request) rest.RestResponse {
	switch r.Method {
	case http.MethodPost:
		return s.postPublicKeyToMonitor(r)
	default:
		return rest.ErrMethodNotAllowed
	}
}

func (s *Server) postPublicKeyToMonitor(r *http.Request) rest.RestResponse {
	var t postPublicKeyToMonitorInput
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		s.logger.Error().WithError(err).Message("error decoding post public key to monitor input")
		return rest.ErrBadRequest
	}

	publicKey, err := domain.NewPublicKeyFromHex(t.PublicKey)
	if err != nil {
		s.logger.Error().WithError(err).Message("error creating a public key")
		return rest.ErrBadRequest
	}

	cmd := app.NewAddPublicKeyToMonitor(publicKey)

	if err := s.app.AddPublicKeyToMonitor.Handle(r.Context(), cmd); err != nil {
		s.logger.Error().WithError(err).Message("error calling the add public key to monitor handler")
		return rest.ErrInternalServerError
	}

	return rest.NewResponse(nil)
}

type postPublicKeyToMonitorInput struct {
	PublicKey string `json:"publicKey"`
}
