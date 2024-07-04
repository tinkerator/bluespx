// Package bluespx connects to and monitors a Spectryx Blue visual
// spectrum analyzer. It displays the current samples on a web page.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/term"
)

var (
	tty          = flag.String("tty", "/dev/ttyUSB0", "tty identifier or device filename")
	baud         = flag.Int("baud", 115200, "preferred baud rate")
	samplePeriod = flag.Duration("period", 1*time.Second, "time between spectrum samples")
	addr         = flag.String("addr", "localhost:8080", "webserver address")
	debug        = flag.Bool("debug", false, "enable for more log output")
)

type Conn struct {
	t *term.Term
	r *bufio.Reader

	mu                       sync.Mutex
	Wavelengths, Intensities []int
}

// Close closes the serial connection.
func (c *Conn) Close() error {
	return c.t.Close()
}

// reset attempts to reset the connection.
func reset(t *term.Term) {
	t.SetDTR(false)
	time.Sleep(250 * time.Millisecond)
	t.SetDTR(true)
}

// newConn returns an opened connection to the tty serial terminal.
func newConn(tty string) (*Conn, error) {
	t, err := term.Open(tty, term.Speed(*baud), term.RawMode)
	if err != nil {
		return nil, err
	}
	term.RawMode(t)
	reset(t)
	c := &Conn{
		t: t,
		r: bufio.NewReaderSize(t, 4096),
	}
	return c, nil
}

// NewConn opens a serial tty. If tty is prefixed with "/" the name is
// assumed to be a device filename. Otherwise, the text is interpreted
// as a substring of a /dev/serial/by-id/* file. In this latter case,
// some effort is taken to ensure the string match is unique and if it
// does not map to a single device, an error is returned.
func NewConn(tty string) (*Conn, error) {
	if strings.HasPrefix(tty, "/") {
		return newConn(tty)
	}
	files, err := ioutil.ReadDir("/dev/serial/by-id")
	if err != nil {
		return nil, err
	}
	var path string
	for _, f := range files {
		if os.ModeSymlink&f.Mode() == 0 {
			continue
		}
		if !strings.Contains(f.Name(), tty) {
			continue
		}
		if path != "" {
			return nil, fmt.Errorf("conflict %q vs %q for selection %q", f.Name(), path, tty)
		}
		path = f.Name()
	}
	if path == "" {
		return nil, fmt.Errorf("no match for %q", tty)
	}
	return newConn("/dev/serial/by-id/" + path)
}

// ReadLine reads a "\n" terminated line from an open connection.
func (c *Conn) ReadLine() (string, error) {
	return c.r.ReadString('\n')
}

// monitor requests samples from the spectrum analyzer device.  The
// very first sample obtains the monitored nm wavelengths.  All
// subsequent lines are measures of intensity. This code was inspired
// by the sample python code (SpectryxBlueViewer.py). However,
// experience using the code yielded some unstable results, so we
// retry until we obtain stable values.
func (c *Conn) monitor() {
	lines := make(chan string, 2)
	go func() {
		defer close(lines)
		initialized := false
		go func() {
			// Unclear how to make sure the device has started.
			// so sleep for 3 seconds.
			time.Sleep(3 * time.Second)
			initialized = true
			fmt.Fprint(c.t, "w")
		}()
		for {
			line, err := c.ReadLine()
			if err != nil {
				log.Fatalf("error: %q: %v", line, err)
				continue
			}
			if *debug {
				log.Printf("got: %q", line)
			}
			if !initialized {
				continue
			}
			if initialized {
				lines <- line
			}
		}
	}()

	first := true
	for line := range lines {
		if *debug {
			log.Printf("sample: %q", line)
		}
		junk := false
		var vs []int
		for _, num := range strings.Split(strings.TrimSpace(line), ",") {
			v, err := strconv.ParseInt(num, 10, 64)
			if err != nil {
				junk = true
				break
			}
			vs = append(vs, int(v))
		}
		if len(vs) != 640 {
			// When the output isn't corrupted, it contains 640 entries.
			junk = true
		}
		time.Sleep(*samplePeriod)
		if !junk {
			c.mu.Lock()
			if first {
				first = false
				c.Wavelengths = vs
			} else {
				if c.Intensities == nil {
					log.Print("sample captured")
				}
				c.Intensities = vs
			}
			c.mu.Unlock()
		} else {
			if first {
				fmt.Fprint(c.t, "w")
				continue
			}
		}
		if *debug {
			log.Printf("numbers [%v]: %d", first, vs)
		}
		// request another sample
		fmt.Fprint(c.t, "s")
	}
}

func (c *Conn) Handler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./"+r.URL.Path)
}

type Request struct {
	Cmd string
}

type Response struct {
	Error  string
	Values []int
}

func (c *Conn) RPC(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/rpc" {
		http.NotFound(w, r)
		return
	}
	var req Request
	var resp Response
	var err error
	defer func() {
		if err != nil {
			resp.Error = err.Error()
		}
		d, _ := json.Marshal(resp)
		w.Write(d)
	}()

	j := r.FormValue("rpc")
	if err = json.Unmarshal([]byte(j), &req); err != nil {
		return
	}

	switch req.Cmd {
	case "scale":
		c.mu.Lock()
		resp.Values = c.Wavelengths
		c.mu.Unlock()
	case "sample":
		c.mu.Lock()
		resp.Values = c.Intensities
		c.mu.Unlock()
	default:
		resp.Error = "unsupported command"
	}
}

func main() {
	flag.Parse()

	c, err := NewConn(*tty)
	if err != nil {
		log.Fatalf("failed to open %q: %v", *tty, err)
	}
	go c.monitor()

	http.HandleFunc("/", c.Handler)
	http.HandleFunc("/rpc", c.RPC)

	log.Fatal(http.ListenAndServe(*addr, nil))
}
