package handler

import (
    "fmt"

    "github.com/geniustag/QuantBot/constant"
    "github.com/geniustag/QuantBot/model"
)

func AuthUser(username string) (user model.User, message string, err error) {
    if username == "" {
        message = constant.ErrAuthorizationError
        return
    }
    user, err = model.GetUser(username)
    if err != nil {
        fmt.Sprint(err)
        return
    }
    return
}