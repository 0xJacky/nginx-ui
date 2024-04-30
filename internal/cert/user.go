package cert

import (
    "github.com/0xJacky/Nginx-UI/model"
)

// User You'll need a user or account type that implements acme.User
type User struct {
    model.AcmeUser
}

func newUser(email string) *User {
    return &User{
        AcmeUser: model.AcmeUser{
            Email: email,
        },
    }
}
