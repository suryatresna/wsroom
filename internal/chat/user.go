package chat

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// User represents user connection.
// It contains logic of receiving and sending messages.
// That is, there are no active reader or writer. Some other layer of the
// application should call Receive() to read user's incoming message.
type User struct {
	io   sync.Mutex
	conn io.ReadWriteCloser

	id   uint
	name string
	chat *Chat
}

// Receive reads next message from user's underlying connection.
// It blocks until full message received.
func (u *User) Receive() error {
	req, err := u.readRequest()
	if err != nil {
		u.conn.Close()
		return err
	}
	if req == nil {
		// Handled some control message.
		return nil
	}
	switch req.Method {
	case "rename":
		name, ok := req.Params["name"].(string)
		if !ok {
			return u.writeErrorTo(req, Object{
				"error": "bad params",
			})
		}
		prev, ok := u.chat.Rename(u, name)
		if !ok {
			return u.writeErrorTo(req, Object{
				"error": "already exists",
			})
		}
		u.chat.Broadcast("rename", Object{
			"prev": prev,
			"name": name,
			"time": timestamp(),
		})
		return u.writeResultTo(req, nil)
	case "publish":
		req.Params["author"] = u.name
		req.Params["time"] = timestamp()
		u.chat.Broadcast("publish", req.Params)
	default:
		return u.writeErrorTo(req, Object{
			"error": "not implemented",
		})
	}
	return nil
}

// readRequests reads json-rpc request from connection.
// It takes io mutex.
func (u *User) readRequest() (*Request, error) {
	u.io.Lock()
	defer u.io.Unlock()

	h, r, err := wsutil.NextReader(u.conn, ws.StateServerSide)
	if err != nil {
		return nil, err
	}
	if h.OpCode.IsControl() {
		return nil, wsutil.ControlFrameHandler(u.conn, ws.StateServerSide)(h, r)
	}

	req := &Request{}
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(req); err != nil {
		return nil, err
	}

	return req, nil
}

func (u *User) writeErrorTo(req *Request, err Object) error {
	return u.write(Error{
		ID:    req.ID,
		Error: err,
	})
}

func (u *User) writeResultTo(req *Request, result Object) error {
	return u.write(Response{
		ID:     req.ID,
		Result: result,
	})
}

func (u *User) writeNotice(method string, params Object) error {
	return u.write(Request{
		Method: method,
		Params: params,
	})
}

func (u *User) write(x interface{}) error {
	w := wsutil.NewWriter(u.conn, ws.StateServerSide, ws.OpText)
	encoder := json.NewEncoder(w)

	u.io.Lock()
	defer u.io.Unlock()

	if err := encoder.Encode(x); err != nil {
		return err
	}

	return w.Flush()
}

func (u *User) writeRaw(p []byte) error {
	u.io.Lock()
	defer u.io.Unlock()

	_, err := u.conn.Write(p)

	return err
}
