package proxy

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func resolveConnectHost(host string) (string, error) {
	_, portStr, err := net.SplitHostPort(host)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return host + ":443", nil
		}
		return "", err
	}
	if _, err := strconv.Atoi(portStr); err != nil {
		return "", err
	}
	return host, nil
}

func hijackConnection(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijacking not supported")
	}
	return hijacker.Hijack()
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, err := resolveConnectHost(r.Host)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	r.Host = host

	clientConn, rw, err := hijackConnection(w)
	if err != nil {
		if err.Error() == "hijacking not supported" {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		} else {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		}
		return
	}

	targetConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		io.WriteString(clientConn, "HTTP/1.1 503 Service Unavailable\r\n\r\n")
		clientConn.Close()
		return
	}

	io.WriteString(clientConn, "HTTP/1.1 200 Connection established\r\n\r\n")

	if buffered := rw.Reader.Buffered(); buffered > 0 {
		peeked, _ := rw.Reader.Peek(buffered)
		targetConn.Write(peeked)
	}

	go s.transfer(targetConn, clientConn)
	go s.transfer(clientConn, targetConn)
}

func (s *Server) transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()

	bufPtr := s.bufferPool.Get().(*[]byte)
	defer s.bufferPool.Put(bufPtr)

	io.CopyBuffer(destination, source, *bufPtr)
}
