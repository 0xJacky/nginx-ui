package cert

type ChannelWriter struct {
    Ch chan []byte
}

func NewChannelWriter() *ChannelWriter {
    return &ChannelWriter{
        Ch: make(chan []byte, 1024),
    }
}

func (cw *ChannelWriter) Write(p []byte) (n int, err error) {
    n = len(p)
    temp := make([]byte, n)
    copy(temp, p)
    cw.Ch <- temp
    return n, nil
}
