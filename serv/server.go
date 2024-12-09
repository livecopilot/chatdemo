package serv

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
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
		"id":     s.id,
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Request received")

		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			log.Error("Error upgrading connection")
			conn.Close() // close the connection
			return
		}

		user := r.URL.Query().Get("user")
		if user == "" {
			log.Error("User not found")
			conn.Close()
			return
		}

		old, ok := s.addUser(user, conn)
		if ok {
			old.Close()
			log.Infof("close old connection %v", old.RemoteAddr())
		}

		log.Infof("user %s in from %v", user, conn.RemoteAddr())

		go func(user string, conn net.Conn) {
			err := s.readloop(user, conn)

			if err != nil {
				log.Warnf("readloop error: %v", err)
			}

			conn.Close()

			s.delUser(user)

			log.Infof("connection of %s closed", user)

		}(user, conn)

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

// addUser
func (s *Server) addUser(user string, conn net.Conn) (net.Conn, bool) {
	s.Lock()
	defer s.Unlock()

	c, ok := s.users[user]
	s.users[user] = conn

	return c, ok
}

// delUser
func (s *Server) delUser(user string) {
	s.Lock()
	defer s.Unlock()

	delete(s.users, user)
}

// readloop
func (s *Server) readloop(user string, conn net.Conn) error {
	for {
		_ = conn.SetReadDeadline(time.Now().Add(s.options.readwait))

		frame, err := ws.ReadFrame(conn)

		if err != nil {
			return err
		}

		if frame.Header.OpCode == ws.OpPing {

			_ = wsutil.WriteServerMessage(conn, ws.OpPong, nil)
			logrus.Info("Pong sent")

			continue
		}

		if frame.Header.OpCode == ws.OpClose {

			return errors.New("Connection closed")
		}

		logrus.Info(frame.Header)

		if frame.Header.Masked {
			ws.Cipher(frame.Payload, frame.Header.Mask, 0)
		}

		if frame.Header.OpCode == ws.OpText {

			go s.handle(user, string(frame.Payload))
		} else if frame.Header.OpCode == ws.OpBinary {
			go s.handleBinary(user, frame.Payload)

		}

	}

}

// handle
func (s *Server) handle(user string, message string) {
	logrus.Infof("recv message %s from %s", message, user)

	s.Lock()
	defer s.Unlock()

	broadcast := fmt.Sprintf("%s   --From %s", message, user)

	for u, conn := range s.users {
		if u == user {
			continue
		}

		logrus.Infof("send message %s to %s", broadcast, u)
		err := s.writeText(conn, broadcast)

		if err != nil {
			logrus.Errorf("Error sending message to %s: %v", u, err)
		}
	}
}

// writeText writes a text message to the connection
func (s *Server) writeText(conn net.Conn, message string) error {
	f := ws.NewTextFrame([]byte(message))

	err := conn.SetWriteDeadline(time.Now().Add(s.options.writewait))
	if err != nil {
		return err
	}

	return ws.WriteFrame(conn, f)

}

const (
	CommandPing = 100
	CommandPong = 101
)

// handleBinary
func (s *Server) handleBinary(user string, message []byte) {
	logrus.Infof("recv binary message %v from %s", message, user)

	s.Lock()
	defer s.Unlock()

	i := 0
	command := binary.BigEndian.Uint16(message[i : i+2])
	i += 2
	playloadLen := binary.BigEndian.Uint32(message[i : i+4])
	logrus.Infof("command: %d, playloadLen: %d", command, playloadLen)

	if command == CommandPing {
		u := s.users[user]

		err := wsutil.WriteServerBinary(u, []byte{0, CommandPong, 0, 0, 0, 0})

		if err != nil {
			logrus.Errorf("Error sending pong to %s: %v", user, err)
		}
	}

}
