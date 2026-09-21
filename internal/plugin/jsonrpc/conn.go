package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"go.uber.org/zap"
)

// MaxMessageSize is the hard limit for a single frame, including the trailing
// newline. Oversize frames are rejected instead of being buffered.
const MaxMessageSize = 4 << 20

// readBufferSize is the initial size of the line reader. Longer frames grow a
// scratch buffer up to the configured maximum.
const readBufferSize = 64 << 10

// Handler serves one inbound method. Returning a *protocol.Error sends that
// exact code and message back to the peer, any other error is reported as
// protocol.CodeInternalError with the error text as the message.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Option configures a Conn.
type Option func(*Conn)

// WithLogger attaches a logger used for frames that cannot be answered.
func WithLogger(l *zap.SugaredLogger) Option {
	return func(c *Conn) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithMaxMessageSize overrides MaxMessageSize. Mainly useful for tests.
func WithMaxMessageSize(n int) Option {
	return func(c *Conn) {
		if n > 0 {
			c.maxSize = n
		}
	}
}

// Conn is a goroutine safe JSON-RPC 2.0 peer over a newline delimited stream.
type Conn struct {
	r io.Reader
	w io.Writer

	// writes feeds the writer goroutine, the only one touching w. A peer that
	// stopped draining its input blocks that goroutine and not the callers,
	// so a call still honours its context.
	writes    chan writeRequest
	writeOnce sync.Once

	mu       sync.Mutex
	handlers map[string]Handler
	pending  map[uint64]chan *message
	nextID   uint64
	closed   bool
	closeErr error

	maxSize int
	logger  *zap.SugaredLogger

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewConn builds a peer that reads frames from r and writes frames to w. The
// connection does nothing until Serve runs.
func NewConn(r io.Reader, w io.Writer, opts ...Option) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Conn{
		r:        r,
		w:        w,
		handlers: make(map[string]Handler),
		pending:  make(map[uint64]chan *message),
		writes:   make(chan writeRequest),
		maxSize:  MaxMessageSize,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Handle registers a handler for an inbound method. Registering the same method
// twice replaces the previous handler, a nil handler removes it.
func (c *Conn) Handle(method string, h Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if h == nil {
		delete(c.handlers, method)
		return
	}
	c.handlers[method] = h
}

// Done is closed once the connection is no longer usable.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Call sends a request and waits for the matching response. result may be nil
// when the reply is not needed. Cancelling ctx aborts the wait and drops the
// pending entry so a late reply is discarded.
func (c *Conn) Call(ctx context.Context, method string, params any, result any) error {
	ch := make(chan *message, 1)

	c.mu.Lock()
	if c.closed {
		err := c.closeErr
		c.mu.Unlock()
		if err == nil {
			err = ErrClosed
		}
		return err
	}
	c.nextID++
	id := c.nextID
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.writeFrame(ctx, &outgoingRequest{JSONRPC: Version, ID: id, Method: method, Params: params}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return c.closeReason()
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result == nil || len(resp.Result) == 0 || isNullJSON(resp.Result) {
			return nil
		}
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("jsonrpc: decode result of %s: %w", method, err)
		}
		return nil
	}
}

// Notify sends a notification. Notifications are never answered, so the call
// returns as soon as the frame is written.
func (c *Conn) Notify(ctx context.Context, method string, params any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.writeFrame(ctx, &outgoingNotification{JSONRPC: Version, Method: method, Params: params})
}

// NotifyBatch sends several notifications as one JSON-RPC batch frame. An
// empty batch is a no-op.
func (c *Conn) NotifyBatch(ctx context.Context, notifications []Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(notifications) == 0 {
		return nil
	}
	batch := make([]*outgoingNotification, 0, len(notifications))
	for _, n := range notifications {
		if n.Method == "" {
			return errors.New("jsonrpc: notification method is empty")
		}
		batch = append(batch, &outgoingNotification{JSONRPC: Version, Method: n.Method, Params: n.Params})
	}
	return c.writeFrame(ctx, batch)
}

// Serve reads frames until the stream ends or ctx is cancelled. Every pending
// call fails once it returns.
func (c *Conn) Serve(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()

	br := bufio.NewReaderSize(c.r, readBufferSize)
	var serveErr error
	for {
		line, err := readLine(br, c.maxSize)
		if len(line) > 0 {
			c.dispatch(line)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				serveErr = err
			}
			break
		}
	}

	c.closeWith(serveErr)
	if serveErr != nil {
		return serveErr
	}
	return ctx.Err()
}

// Close shuts the connection down and fails every pending call. The reader and
// the writer are closed when they support it, which unblocks a Serve loop that
// is sitting on a blocking read.
func (c *Conn) Close() error {
	c.closeWith(nil)
	return nil
}

func (c *Conn) closeWith(cause error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	if cause != nil {
		c.closeErr = cause
	} else {
		c.closeErr = ErrClosed
	}
	pending := c.pending
	c.pending = make(map[uint64]chan *message)
	c.mu.Unlock()

	for _, ch := range pending {
		close(ch)
	}

	c.cancel()
	close(c.done)

	if rc, ok := c.r.(io.Closer); ok {
		_ = rc.Close()
	}
	if wc, ok := c.w.(io.Closer); ok {
		_ = wc.Close()
	}
}

func (c *Conn) closeReason() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closeErr != nil {
		return c.closeErr
	}
	return ErrClosed
}

// writeRequest is one frame handed to the writer goroutine.
type writeRequest struct {
	data []byte
	done chan error
}

// writeFrame queues one frame and waits for it to be written, unless ctx ends
// or the connection closes first. A frame the writer already took is still
// written whole, so frames never interleave.
func (c *Conn) writeFrame(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("jsonrpc: encode frame: %w", err)
	}
	if len(data)+1 > c.maxSize {
		return ErrMessageTooLarge
	}
	data = append(data, '\n')

	c.writeOnce.Do(func() { go c.writeLoop() })
	request := writeRequest{data: data, done: make(chan error, 1)}
	select {
	case c.writes <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.closeReason()
	}
	select {
	case err = <-request.done:
		if err != nil {
			return fmt.Errorf("jsonrpc: write frame: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.closeReason()
	}
}

// writeLoop writes the queued frames one after the other until the connection
// closes. Close closes the writer as well, which unblocks a write the peer
// never drains.
func (c *Conn) writeLoop() {
	for {
		select {
		case request := <-c.writes:
			_, err := c.w.Write(request.data)
			request.done <- err
		case <-c.done:
			return
		}
	}
}

// dispatch routes one raw line. Batch arrays are unpacked and every element is
// routed on its own, so replies are emitted as individual frames.
func (c *Conn) dispatch(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}

	if line[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(line, &batch); err != nil {
			c.replyParseError(line)
			return
		}
		if len(batch) == 0 {
			c.reply(&outgoingResponse{JSONRPC: Version, ID: nullID, Error: Errorf(protocol.CodeInvalidRequest, "empty batch")})
			return
		}
		for _, raw := range batch {
			c.dispatchOne(raw)
		}
		return
	}

	c.dispatchOne(line)
}

func (c *Conn) dispatchOne(raw json.RawMessage) {
	var msg message
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.replyParseError(raw)
		return
	}

	switch {
	case msg.isResponse():
		c.deliver(&msg)
	case msg.isRequest():
		c.serveRequest(&msg)
	case msg.isNotification():
		c.serveNotification(&msg)
	default:
		c.reply(&outgoingResponse{
			JSONRPC: Version,
			ID:      nullID,
			Error:   Errorf(protocol.CodeInvalidRequest, "frame is neither a request, a notification nor a response"),
		})
	}
}

// replyParseError answers unparseable input with a null id, which is what
// JSON-RPC 2.0 asks for. Stray text a plugin prints on stdout ends up here too,
// so the line is logged as well.
func (c *Conn) replyParseError(raw []byte) {
	c.logf("jsonrpc: unparseable frame of %d bytes", len(raw))
	c.reply(&outgoingResponse{JSONRPC: Version, ID: nullID, Error: Errorf(protocol.CodeParseError, "invalid JSON")})
}

func (c *Conn) deliver(msg *message) {
	var id uint64
	if err := json.Unmarshal(msg.ID, &id); err != nil {
		c.logf("jsonrpc: response carries a malformed id %s", string(msg.ID))
		return
	}

	c.mu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()

	if !ok {
		c.logf("jsonrpc: response for unknown or cancelled call id %d", id)
		return
	}
	ch <- msg
}

func (c *Conn) serveRequest(msg *message) {
	handler, ok := c.handler(msg.Method)
	id := append(json.RawMessage(nil), msg.ID...)
	if !ok {
		c.reply(&outgoingResponse{
			JSONRPC: Version,
			ID:      id,
			Error:   Errorf(protocol.CodeMethodNotFound, "unknown method "+msg.Method),
		})
		return
	}

	params := append(json.RawMessage(nil), msg.Params...)
	go func() {
		result, err := invoke(c.ctx, handler, params)
		if err != nil {
			c.reply(&outgoingResponse{JSONRPC: Version, ID: id, Error: toProtocolError(err)})
			return
		}
		if result == nil {
			result = protocol.EmptyResult{}
		}
		c.reply(&outgoingResponse{JSONRPC: Version, ID: id, Result: result})
	}()
}

func (c *Conn) serveNotification(msg *message) {
	handler, ok := c.handler(msg.Method)
	if !ok {
		c.logf("jsonrpc: no handler for notification %s", msg.Method)
		return
	}
	params := append(json.RawMessage(nil), msg.Params...)
	method := msg.Method
	go func() {
		if _, err := invoke(c.ctx, handler, params); err != nil {
			c.logf("jsonrpc: notification %s failed: %v", method, err)
		}
	}()
}

func (c *Conn) handler(method string) (Handler, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	h, ok := c.handlers[method]
	return h, ok
}

// invoke runs a handler and converts a panic into an internal error so one bad
// handler cannot take the whole process down.
func invoke(ctx context.Context, h Handler, params json.RawMessage) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = Errorf(protocol.CodeInternalError, fmt.Sprintf("handler panicked: %v", r))
		}
	}()
	return h(ctx, params)
}

func toProtocolError(err error) *protocol.Error {
	if perr, ok := AsProtocolError(err); ok {
		return perr
	}
	return Errorf(protocol.CodeInternalError, err.Error())
}

func (c *Conn) reply(resp *outgoingResponse) {
	err := c.writeFrame(c.ctx, resp)
	if err != nil && !errors.Is(err, ErrClosed) && !errors.Is(err, context.Canceled) {
		c.logf("jsonrpc: cannot send response: %v", err)
	}
}

func (c *Conn) logf(format string, args ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Debugf(format, args...)
}

// readLine reads one newline terminated frame, refusing anything longer than
// max. A frame that is too long is unrecoverable because the rest of the line
// would be parsed as a new frame, so the caller stops reading.
func readLine(br *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := br.ReadSlice('\n')
		if len(buf)+len(chunk) > max {
			return nil, ErrMessageTooLarge
		}
		buf = append(buf, chunk...)
		switch {
		case err == nil:
			return buf, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return buf, err
		}
	}
}
