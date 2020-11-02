package elasticwriter

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/tsaarni/11th-init/internal/pkg/ringbuffer"
)

// ElasticWriter streams logs to elasticsearch
type ElasticWriter struct {
	server string
	cert   string
	pkey   string
	ca     string

	mutex    *sync.Mutex
	events   *ringbuffer.RingBuffer
	readable chan struct{}

	stop     chan struct{}
	finished chan struct{}
}

// New creates new ElasticWriter
func New(server, certificate, privateKey, caCert string, numEvents int) (*ElasticWriter, error) {
	w := &ElasticWriter{
		server: server,
		cert:   certificate,
		pkey:   privateKey,
		ca:     caCert,

		mutex:    &sync.Mutex{},
		events:   ringbuffer.New(numEvents),
		readable: make(chan struct{}, 1),

		stop:     make(chan struct{}, 1),
		finished: make(chan struct{}, 1),
	}

	// Try loading TLS configuration in order to get early error if files are missing
	_, err := w.tlsConfig()
	if err != nil {
		return nil, err
	}

	// Connect to Elasticsearch and start forwarding log events
	go w.process()

	return w, nil
}

func (w *ElasticWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	empty := w.events.Empty()
	w.events.Push(p)

	// If ring buffer was empty, signal the process() go routine that there is values to read.
	if empty {
		// Non-blocking send.
		select {
		case w.readable <- struct{}{}:
			// Message sent
		default:
			// Message already in channel
		}
	}
	return len(p), nil
}

// Close will shutdown the writer
func (w *ElasticWriter) Close() {
	log.Println("Cancelling ElasticSearchWriter")

	// Signal the process() goroutine to stop and wait for it.
	w.stop <- struct{}{}
	<-w.finished
}

func (w *ElasticWriter) process() {
	log.Println("Starting ElasticsearchWriter")

CONNECT_LOOP:
	for {
		log.Println("Establishing connection to:", w.server)
		conn, err := w.dial()
		if err != nil {
			w.backoff(err)
			continue
		}
		log.Println("Connection established")

		for {
			log.Println("Waiting for events")

			select {
			case <-w.readable:
				log.Println("Events received")
			case <-w.stop:
				break CONNECT_LOOP
			}

			w.mutex.Lock()
			event, err := w.events.Pop()
			w.mutex.Unlock()
			if err != nil {
				continue
			}

			log.Println("Writing event to Elasticsearch")
			_, err = conn.Write(event.([]byte))
			if err != nil {
				log.Println("Write failed:", err)
				conn.Close()
				continue CONNECT_LOOP
			}
		}

	}

	log.Println("Process loop exiting")
	w.finished <- struct{}{}
}

func (w *ElasticWriter) dial() (*tls.Conn, error) {
	tlsConfig, err := w.tlsConfig()
	if err != nil {
		return nil, err
	}

	return tls.Dial("tcp", w.server, tlsConfig)
}

func (w *ElasticWriter) tlsConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(w.cert, w.pkey)
	if err != nil {
		return nil, err
	}

	caCerts := x509.NewCertPool()
	pemData, err := ioutil.ReadFile(w.ca)
	if err != nil {
		return nil, err
	}

	caCerts.AppendCertsFromPEM(pemData)
	return &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: caCerts}, nil
}

func (w *ElasticWriter) backoff(err error) {
	timeout := 1 * time.Second
	log.Printf("Error %s, sleeping for %s", err, timeout)
	time.Sleep(1 * time.Second)
}
