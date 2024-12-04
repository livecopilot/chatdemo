package serv

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type ServerOptions struct {
	writewait time.Duration
	readwait  time.Duration
}

type Server struct {
	once    sync.Once
	options ServerOptions
	id      string
	address string
	sync.Mutex

	users map[string]net.Conn // map of users
}

func newServer(id, address string) *Server {
	return &Server{
		id:      id,
		address: address,
		users:   make(map[string]net.Conn, 100),
		options: ServerOptions{
			writewait: 10 * time.Second,
			readwait:  2 * time.Minute,
		},
	}
}

func NewServer(id, address string) *Server {
	return newServer(id, address)
}

// Start starts the server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	log := logrus.WithFields(logrus.Fields{
		"module": "server",
		"listen": s.address,
		"id":	 s.id,
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Request received")
	})

	log.Info("Starting server.")

	return http.ListenAndServe(s.address, mux)
}

// Shutdown
func (s *Server) Shutdown() {
	s.once.Do(func() {
		s.Lock()
		defer s.Unlock()

		for _, conn := range s.users {
			conn.Close()
		}
	})
}
