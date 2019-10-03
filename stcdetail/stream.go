package stcdetail

import (
	"bufio"
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// A status line for a non-200 HTTP response
type HTTPerror struct {
	Resp *http.Response
	Body []byte
}

func NewHTTPerror(resp *http.Response) *HTTPerror {
	var body []byte
	if resp.Body != nil {
		body, _ = ioutil.ReadAll(resp.Body)
	}
	return &HTTPerror {
		Resp: resp,
		Body: body,
	}
}

func (e *HTTPerror) Error() string {
	return e.Resp.Status
}

// Returns true for 503, false otherwise.  Should examine the result
// more carefully to distinguish between transient or permanent 500
// errors.
func (e *HTTPerror) Temporary() bool {
	switch e.Resp.StatusCode {
	case 503:
		return true
	}
	return false
}

type streamEvent struct {
	Type  string
	Data  []byte
	Id    []byte
	Retry *int64
}

func (e *streamEvent) reset() {
	e.Type = "message"
	e.Data = nil
}

func parseKV(line []byte) (string, []byte) {
	if len(line) == 0 {
		return "", nil
	}
	kv := bytes.SplitN(line, []byte(":"), 2)
	if len(kv) == 1 {
		kv = append(kv, []byte{})
	}
	if len(kv[1]) > 0 && kv[1][0] == ' ' {
		kv[1] = kv[1][1:]
	}
	return string(kv[0]), kv[1]
}

func (e *streamEvent) interpret(line []byte) bool {
	if len(line) == 0 {
		if len(e.Data) > 0 {
			// Spec says chop off final newline
			e.Data = e.Data[:len(e.Data)-1]
		}
		return false
	}
	switch k, v := parseKV(line); k {
	case "event":
		e.Type = string(v)
	case "data":
		e.Data = append(e.Data, v...)
		e.Data = append(e.Data, '\n')
	case "id":
		e.Id = v
	case "retry":
		if i, err := strconv.ParseInt(string(v), 10, 32); err == nil {
			e.Retry = &i
		}
	}
	return true
}

/*

Stream results from a URL that returns a body of type
text/event-stream, as described in https://www.w3.org/TR/eventsource/

The callback function, cb, gets called with a series of event types
and data payloads.  The event type will generally be one of "message",
"error", "open", or "close" (where you will typically ignore events of
type open and close).

Stream does not spawn a new goroutine.  It loops until the Context ctx
is canceled or there is a non-nil error.  Hence, the cb can make
Stream return by returning a non-nil error.  You will generally want
to spawn Stream in a new goroutine, and may wish to wrap it in a loop
to keep trying in the face of errors.

In keeping with the Stellar Horizon REST API, if url does not contain
a cursor parameter, Stream adds cursor=now to the query.  It
furthermore updates the cursor to the latest event ID whenever it
needs to reconnect.

*/
func Stream(ctx context.Context, url string,
	cb func(eventType string, data []byte) error) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	} else {
		ctx = context.Background()
	}
	req.Header.Set("Accept", "text/event-stream")
	q := req.URL.Query()
	if _, ok := q["cursor"]; !ok {
		q.Set("cursor", "now")
		req.URL.RawQuery = q.Encode()
	}

	var resp *http.Response
	cleanup := func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
	defer cleanup()

	for ctx.Err() == nil {
		cleanup()
		resp, err = http.DefaultClient.Do(req)
		if err != nil || ctx.Err() != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return NewHTTPerror(resp)
		}
		body := bufio.NewScanner(resp.Body)

		var event streamEvent
		event.reset()
		for body.Scan() {
			if !event.interpret(body.Bytes()) {
				if err = cb(event.Type, event.Data); err != nil {
					return err
				}
				event.reset()
			}
		}

		if len(event.Id) > 0 {
			q.Set("cursor", string(event.Id))
			req.URL.RawQuery = q.Encode()
		}
		if event.Retry != nil {
			select {
			case <-ctx.Done():
			case <-time.After(time.Duration(*event.Retry) * time.Millisecond):
			}
		}
	}
	return nil
}
