package cert

import (
    "crypto"
    "github.com/go-acme/lego/v4/registration"
)

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

// User You'll need a user or account type that implements acme.User
type User struct {
    Email        string
    Registration *registration.Resource
    Key          crypto.PrivateKey
}

func (u *User) GetEmail() string {
    return u.Email
}

func (u *User) GetRegistration() *registration.Resource {
    return u.Registration
}

func (u *User) GetPrivateKey() crypto.PrivateKey {
    return u.Key
}
