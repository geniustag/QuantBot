package handler

import (
    "fmt"
    "time"
    "net"

    "github.com/hprose/hprose-golang/rpc"
    "github.com/geniustag/QuantBot/constant"
    "github.com/geniustag/QuantBot/model"
    "github.com/geniustag/QuantBot/trader"
)

type runner struct{}

// List
func (runner) List(algorithmID int64, ctx rpc.Context) (resp response) {
    username := ctx.GetString("username")
    if username == "" {
        resp.Message = constant.ErrAuthorizationError
        return
    }
    self, err := model.GetUser(username)
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    traders, err := self.ListTrader(algorithmID)
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    for i, t := range traders {
        traders[i].Status = trader.GetTraderStatus(t.ID)
    }
    resp.Data = traders
    resp.Success = true
    return
}

// Put
func (runner) Put(req model.Trader, ctx rpc.Context) (resp response) {
    username := ctx.GetString("username")
    if username == "" {
        resp.Message = constant.ErrAuthorizationError
        return
    }
    self, err := model.GetUser(username)
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    db, err := model.NewOrm()
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    defer db.Close()
    db = db.Begin()
    req.LastRunAt = time.Now()
    req.ServerIp = getLocalIp()
    if req.ID > 0 {
        if err := self.UpdateTrader(req); err != nil {
            resp.Message = fmt.Sprint(err)
            return
        }
        resp.Success = true
        return
    }
    req.UserID = self.ID
    if err := db.Create(&req).Error; err != nil {
        db.Rollback()
        resp.Message = fmt.Sprint(err)
        return
    }
    for _, e := range req.Exchanges {
        traderExchange := model.TraderExchange{
            TraderID:   req.ID,
            ExchangeID: e.ID,
        }
        if err := db.Create(&traderExchange).Error; err != nil {
            db.Rollback()
            resp.Message = fmt.Sprint(err)
            return
        }
    }
    if err := db.Commit().Error; err != nil {
        db.Rollback()
        resp.Message = fmt.Sprint(err)
        return
    }
    resp.Success = true
    return
}

// Delete
func (runner) Delete(req model.Trader, ctx rpc.Context) (resp response) {
    username := ctx.GetString("username")
    if username == "" {
        resp.Message = constant.ErrAuthorizationError
        return
    }
    self, err := model.GetUser(username)
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    if req, err = self.GetTrader(req.ID); err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    if err := model.DB.Where("id = ?", req.ID).Delete(&model.Trader{}).Error; err != nil {
        resp.Message = fmt.Sprint(err)
    } else {
        resp.Success = true
    }
    return
}

// Switch
func (runner) Switch(req model.Trader, ctx rpc.Context) (resp response) {
    username := ctx.GetString("username")
    if username == "" {
        resp.Message = constant.ErrAuthorizationError
        return
    }
    self, err := model.GetUser(username)
    if err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    if req, err = self.GetTrader(req.ID); err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    if err := trader.Switch(self, req.ID); err != nil {
        resp.Message = fmt.Sprint(err)
        return
    }
    resp.Success = true
    return
}

func getLocalIp() string {
    addrSlice, err := net.InterfaceAddrs()
    if nil != err {
        return "localhost"
    }
    for _, addr := range addrSlice {
        if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if nil != ipnet.IP.To4() {
                return ipnet.IP.String()
            }
        }
    }
    return "localhost"
}

