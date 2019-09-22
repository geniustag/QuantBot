package handler

import (
    "fmt"

    "github.com/hprose/hprose-golang/rpc"
    "github.com/geniustag/QuantBot/model"
)

type algorithm struct{}

// List ...
func (algorithm) List(size, page int64, order string, ctx rpc.Context) (resp response) {
    user, message, err := AuthUser(ctx.GetString("username"))
    if err != nil {
        resp.Message = message
        return
    }
    total, algorithms, err := user.ListAlgorithm(size, page, order)
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    resp.Data = struct {
        Total int64
        List  []model.Algorithm
    }{
        Total: total,
        List:  algorithms,
    }
    resp.Success = true
    return
}

// Put
func (algorithm) Put(req model.Algorithm, ctx rpc.Context) (resp response) {
    user, message, err := AuthUser(ctx.GetString("username"))
    if err != nil {
        resp.Message = message
        return
    }
    algorithm := req
    if req.ID > 0 {
        if err := model.DB.First(&algorithm, req.ID).Error; err != nil {
            resp.Message = fmt.Sprint(err)
            return
        }
        algorithm.Name = req.Name
        algorithm.Description = req.Description
        algorithm.Script = req.Script
        algorithm.EvnDefault = req.EvnDefault
        if err := model.DB.Save(&algorithm).Error; err != nil {
            resp.Message = fmt.Sprint(err)
            return
        }
        resp.Success = true
        return
    }
    req.UserID = user.ID
    if err := model.DB.Create(&req).Error; err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    resp.Success = true
    return
}

// Delete
func (algorithm) Delete(ids []int64, ctx rpc.Context) (resp response) {
    user, message, err := AuthUser(ctx.GetString("username"))
    if err != nil {
        resp.Message = message
        return
    }
    userIds := []int64{}
    _, users, err := user.ListUser(-1, 1, "id")
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    for _, u := range users {
        userIds = append(userIds, u.ID)
    }
    if err := model.DB.Where("id in (?) AND user_id in (?)", ids, userIds).Delete(&model.Algorithm{}).Error; err != nil {
        resp.Message = fmt.Sprint(err)
    } else {
        resp.Success = true
    }
    return
}
