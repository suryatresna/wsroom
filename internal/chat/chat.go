package chat

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type GopoolInterface interface {
	Schedule(task func())
	ScheduleTimeout(timeout time.Duration, task func()) error
	Add(conn net.Conn) error
	Remove(conn net.Conn) error
	Wait() ([]net.Conn, error)
}

// Object represents generic message parameters.
// In real-world application it is better to avoid such types for better
// performance.
type Object map[string]interface{}

type Request struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params Object `json:"params"`
}

type Response struct {
	ID     int    `json:"id"`
	Result Object `json:"result"`
}

type Error struct {
	ID    int    `json:"id"`
	Error Object `json:"error"`
}

// Chat contains logic of user interaction.
type Chat struct {
	mu  sync.RWMutex
	seq uint
	us  []*User
	ns  map[string]*User

	pool GopoolInterface
	out  chan []byte
}

// NewChat initiate chat
func NewChat(pool GopoolInterface) *Chat {
	chat := &Chat{
		pool: pool,
		ns:   make(map[string]*User),
		out:  make(chan []byte, 1),
	}

	go chat.writer()

	return chat
}

// Register registers new connection as a User.
func (c *Chat) Register(conn net.Conn) *User {
	user := &User{
		chat: c,
		conn: conn,
	}

	c.mu.Lock()
	{
		user.id = c.seq
		user.name = c.randName()

		c.us = append(c.us, user)
		c.ns[user.name] = user

		c.seq++
	}
	c.mu.Unlock()

	user.writeNotice("hello", Object{
		"name": user.name,
	})
	c.Broadcast("greet", Object{
		"name": user.name,
		"time": timestamp(),
	})

	return user
}

// Remove removes user from chat.
func (c *Chat) Remove(user *User) {
	c.mu.Lock()
	removed := c.remove(user)
	c.mu.Unlock()

	if !removed {
		return
	}

	c.Broadcast("goodbye", Object{
		"name": user.name,
		"time": timestamp(),
	})
}

// Rename renames user.
func (c *Chat) Rename(user *User, name string) (prev string, ok bool) {
	c.mu.Lock()
	{
		if _, has := c.ns[name]; !has {
			ok = true
			prev, user.name = user.name, name
			delete(c.ns, prev)
			c.ns[name] = user
		}
	}
	c.mu.Unlock()

	return prev, ok
}

// Broadcast sends message to all alive users.
func (c *Chat) Broadcast(method string, params Object) error {
	var buf bytes.Buffer

	w := wsutil.NewWriter(&buf, ws.StateServerSide, ws.OpText)
	encoder := json.NewEncoder(w)

	r := Request{Method: method, Params: params}
	if err := encoder.Encode(r); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}

	c.out <- buf.Bytes()

	return nil
}

// writer writes broadcast messages from chat.out channel.
func (c *Chat) writer() {
	for bts := range c.out {
		c.mu.RLock()
		us := c.us
		c.mu.RUnlock()

		for _, u := range us {
			u := u // For closure.
			c.pool.Schedule(func() {
				u.writeRaw(bts)
			})
		}
	}
}

// mutex must be held.
func (c *Chat) remove(user *User) bool {
	if _, has := c.ns[user.name]; !has {
		return false
	}

	delete(c.ns, user.name)

	i := sort.Search(len(c.us), func(i int) bool {
		return c.us[i].id >= user.id
	})
	if i >= len(c.us) {
		panic("chat: inconsistent state")
	}

	without := make([]*User, len(c.us)-1)
	copy(without[:i], c.us[:i])
	copy(without[i:], c.us[i+1:])
	c.us = without

	return true
}

func (c *Chat) randName() string {
	var suffix string
	for {
		name := animals[rand.Intn(len(animals))] + suffix
		if _, has := c.ns[name]; !has {
			return name
		}
		suffix += strconv.Itoa(rand.Intn(10))
	}
	return ""
}

func timestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
