package gohttpserv

import (
	"errors"
	"github.com/namsral/flag"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"os/signal"
	"syscall"
)

var (
	Config  = flag.String("config", "", "Configuration file")
	Socket  = flag.String("socket", "tcp", "Network socket to use (tcp/unix)")
	Addr    = flag.String("addr", ":5000", "Address to listen / socket filename")
	Proto   = flag.String("proto", "http", "Protocol to use (http/fcgi)")
	Logfile = flag.String("logfile", "", "Log file path")
)

type LoggedMux struct {
	*http.ServeMux
}

//Writes request data to log and passes it to underlying ServeMux
func (mux LoggedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	mux.ServeMux.ServeHTTP(w, r)
}

//Redirects stdout & stderr to Logfile and starts http/fcgi server on
//specified port or socket.
//Server handles SIGINT, SIGKILL & SIGTERM to exit cleanly.
//Request log is written to logfile specified via commandline option.
func Serve(mux *http.ServeMux) error {
	flag.Parse()

	//Redirect stdout & stderr to logfile
	if *Logfile != "" {
		f, err := os.OpenFile(*Logfile,
			os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		fd := int(f.Fd())
		syscall.Dup2(fd, int(os.Stderr.Fd()))
		syscall.Dup2(fd, int(os.Stdout.Fd()))
	}

	listener, err := net.Listen(*Socket, *Addr)
	if err != nil {
		return err
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func(c chan os.Signal) {
		sig := <-c
		log.Printf("Caught signal %s: shutting down.", sig)
		listener.Close()
		os.Exit(0)
	}(sigc)

	if mux == nil {
		mux = http.DefaultServeMux
	}
	loggedMux := LoggedMux{mux}

	log.Printf("Staring server")
	switch *Proto {
	case "http":
		err = http.Serve(listener, loggedMux)
	case "fcgi":
		err = fcgi.Serve(listener, loggedMux)
	default:
		err = errors.New("Unknown protocol " + *Proto)
	}
	return err
}
