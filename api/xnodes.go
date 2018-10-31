package api

import (
    "fmt"
    "strconv"
    "strings"
    "time"
    "encoding/json"

    "github.com/bitly/go-simplejson"
    "github.com/miaolz123/conver"
    "github.com/geniustag/QuantBot/constant"
    "github.com/geniustag/QuantBot/model"
    "io/ioutil"
    "net/http"
)

// OKEX the exchange struct of okex.com
type Xnodes struct {
    stockTypeMap     map[string]string
    tradeTypeMap     map[string]string
    recordsPeriodMap map[string]string
    recordsPeriodMapV3 map[string]string
    minAmountMap     map[string]float64
    records          map[string][]Record
    logger           model.Logger
    host             string
    option           Option

    limit     float64
    lastSleep int64
    lastTimes int64
}

// NewOKEX create an exchange struct of okex.com
func NewXnodes(opt Option) Exchange {
    return &Xnodes{
        stockTypeMap: map[string]string{
            "BTC/USDT":  "btc_usdt",
            "ETH/USDT":  "eth_usdt",
            "EOS/USDT":  "eos_usdt",
            "GST/ETH":   "gst_eth",
            "GST/BTC":  "gst_btc",
            "GST/USDT":  "gst_usdt",
        },
        tradeTypeMap: map[string]string{
            "buy":         constant.TradeTypeBuy,
            "sell":        constant.TradeTypeSell,
            "buy_market":  constant.TradeTypeBuy,
            "sell_market": constant.TradeTypeSell,
        },
        recordsPeriodMap: map[string]string{
            "M":   "1min",
            "M5":  "5min",
            "M15": "15min",
            "M30": "30min",
            "H":   "1hour",
            "D":   "1day",
            "W":   "1week",
        },
        recordsPeriodMapV3: map[string]string{
            "M":   "60",
            "M5":  "300",
            "M15": "900",
            "M30": "1800",
            "H":   "3600",
            "D":   "86400",
            "W":   "604800",
        },
        minAmountMap: map[string]float64{
            "BTC/USDT":  0.001,
            "ETH/USDT":  0.001,
            "EOS/USDT":  0.001,
            "GST/USDT":   100,
            "GST/BTC":   100,
            "GST/ETH":   100,
        },
        records: make(map[string][]Record),
        logger:  model.Logger{TraderID: opt.TraderID, ExchangeType: opt.Type},
        option:  opt,
        host: "https://www.xnodes.pro/api/spot/v3",
        limit:     10.0,
        lastSleep: time.Now().UnixNano(),
    }
}


// Log print something to console
func (e *Xnodes) Log(msgs ...interface{}) {
    e.logger.Log(constant.INFO, "", 0.0, 0.0, msgs...)
}

// GetType get the type of this exchange
func (e *Xnodes) GetType() string {
    return e.option.Type
}

// GetName get the name of this exchange
func (e *Xnodes) GetName() string {
    return e.option.Name
}

// SetLimit set the limit calls amount per second of this exchange
func (e *Xnodes) SetLimit(times interface{}) float64 {
    e.limit = conver.Float64Must(times)
    return e.limit
}

// AutoSleep auto sleep to achieve the limit calls amount per second of this exchange
func (e *Xnodes) AutoSleep() {
    now := time.Now().UnixNano()
    interval := 1e+9/e.limit*conver.Float64Must(e.lastTimes) - conver.Float64Must(now-e.lastSleep)
    if interval > 0.0 {
        time.Sleep(time.Duration(conver.Int64Must(interval)))
    }
    e.lastTimes = 0
    e.lastSleep = now
}

// GetMinAmount get the min trade amonut of this exchange
func (e *Xnodes) GetMinAmount(stock string) float64 {
    return e.minAmountMap[stock]
}

// GetAccount get the account detail of this exchange
func (e *Xnodes) GetAccount() interface{} {
    json, err := e.getAuthJSON("/accounts")
    if err != nil {
        fmt.Println("GET Accounts Error: ", err)
        return false
    }

    currencyFrozens := make(map[string]float64)

    if len(json.MustArray()) > 0 {
        fmt.Println("GET Accounts OK")
        for i := len(json.MustArray()); i > 0; i-- {
            recordJSON := json.GetIndex(i - 1)
            currency := recordJSON.Get("currency").MustString()
            currencyFrozens[currency] = conver.Float64Must(recordJSON.Get("available"))
            currencyFrozens["Frozen" + currency] = conver.Float64Must(recordJSON.Get("hold"))
            fmt.Println("GET Account: " + currency)
        }
    } else {
        fmt.Println("Empty Accounts")
    }

    return currencyFrozens
}

// Trade place an order
 func (e *Xnodes) Trade(tradeType string, stockType string, _price, _amount interface{}, msgs ...interface{}) interface{} {
     stockType = strings.ToUpper(stockType)
     tradeType = strings.ToUpper(tradeType)
     price := conver.Float64Must(_price)
     amount := conver.Float64Must(_amount)
     if _, ok := e.stockTypeMap[stockType]; !ok {
         e.logger.Log(constant.ERROR, "", 0.0, 0.0, "Trade() error, unrecognized stockType: ", stockType)
         return false
     }
     switch tradeType {
     case constant.TradeTypeBuy:
         return e.buy(stockType, price, amount, msgs...)
     case constant.TradeTypeSell:
         return e.sell(stockType, price, amount, msgs...)
     default:
         e.logger.Log(constant.ERROR, "", 0.0, 0.0, "Trade() error, unrecognized tradeType: ", tradeType)
         return false
     }
 }


func (e *Xnodes) postOrder(stockType string, side string, price, amount float64, msgs ...interface{}) interface{} {
    // {"client_oid":"20180728","instrument_id":"btc-usdt","side":"buy","type":"limit","size":"0.1"," notional ":"100","margin_trading ":"1"}
    params := make(map[string]interface{})
    params["client_oid"] = IsoTime();
    params["instrument_id"] = e.stockTypeMap[stockType]
    params["side"] = side
    params["size"] = strconv.FormatFloat(amount, 'E', -1, 32)
    params["price"] = strconv.FormatFloat(price, 'E', -1, 32)

    if price > 0 {
        params["type"] = "limit"
    } else {
        params["type"] = "market"
    }

    bytesData, err := json.Marshal(params)

    jsonBody := string(bytesData)
    json, err := e.postAuthJSON("/orders", jsonBody)

    fmt.Println("Create Order With: " + jsonBody)

    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "Buy() error, ", err)
        return false
    }
    if result := json.Get("result").MustBool(); !result {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "Buy() error, the error number is ", json.Get("code").MustInt())
        return false
    }
    e.logger.Log(constant.BUY, stockType, price, amount, msgs...)
    return fmt.Sprint(json.Get("order_id").Interface())
}

func (e *Xnodes) buy(stockType string, price, amount float64, msgs ...interface{}) interface{} {
   return e.postOrder(stockType, "buy", price, amount)
}

func (e *Xnodes) sell(stockType string, price, amount float64, msgs ...interface{}) interface{} {
   return e.postOrder(stockType, "sell", price, amount)
}

// func (e *Xnodes) GetOrder(stockType string, id string) interface{} {
func (e *Xnodes) GetOrder(stockType, id string) interface{} {

    stockType = strings.ToUpper(stockType)
    if _, ok := e.stockTypeMap[stockType]; !ok {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrder() error, unrecognized stockType: ", stockType)
        return false
    }
    json, err := e.getAuthJSON("/orders/" + id)
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrder() error, ", err)
        return false
    }
    if result := json.Get("result").MustBool(); !result {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrder() error, the error number is ", json.Get("code").MustInt())
        return false
    }
    return Order{
        ID:         fmt.Sprint(json.Get("order_id").Interface()),
        Price:      conver.Float64Must(json.Get("price").MustString()),
        Amount:     conver.Float64Must(json.Get("size").MustString()),
        DealAmount: conver.Float64Must(json.Get("filled_size").MustString()),
        TradeType:  e.tradeTypeMap[json.Get("side").MustString()],
        Currency:   json.Get("instrument_id").MustString(),
        StockType:  stockType,
    }
}

// GetOrders get all unfilled orders
func (e *Xnodes) GetOrders(stockType string) interface{} {
    stockType = strings.ToUpper(stockType)
    if _, ok := e.stockTypeMap[stockType]; !ok {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrders() error, unrecognized stockType: ", stockType)
        return false
    }
    json, err := e.getAuthJSON("/orders_pending?instrument_id="+e.stockTypeMap[stockType])
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrders() error, ", err)
        return false
    }
     //if result := json.Get("result").MustBool(); !result {
     //   e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrders() error, the error number is ", json.Get("code").MustInt())
     //   return false
    // }
    orders := []Order{}
    count := len(json.MustArray())
    for i := 0; i < count; i++ {
        orderJSON := json.GetIndex(i)
        orders = append(orders, Order{
            ID:         fmt.Sprint(orderJSON.Get("order_id").Interface()),
            Price:      conver.Float64Must(orderJSON.Get("price").MustString()),
            Amount:     conver.Float64Must(orderJSON.Get("size").MustString()),
            DealAmount: conver.Float64Must(orderJSON.Get("filled_size").MustString()),
            TradeType:  e.tradeTypeMap[orderJSON.Get("side").MustString()],
            Currency:   orderJSON.Get("instrument_id").MustString(),
            StockType:  stockType,
        })
    }
    return orders
}

// GetTrades get all filled orders recently
func (e *Xnodes) GetTrades(stockType string) interface{} {
   stockType = strings.ToUpper(stockType)
    if _, ok := e.stockTypeMap[stockType]; !ok {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrders() error, unrecognized stockType: ", stockType)
        return false
    }
    json, err := e.getAuthJSON("/orders?instrument_id="+e.stockTypeMap[stockType]+"&status=filled")
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrders() error, ", err)
        return false
    }
    // if result := json.Get("result").MustBool(); !result {
    //     e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetOrders() error, the error number is ", json.Get("code").MustInt())
    //     return false
    // }
    orders := []Order{}
    count := len(json.MustArray())
    for i := 0; i < count; i++ {
        orderJSON := json.GetIndex(i)
        orders = append(orders, Order{
            ID:         fmt.Sprint(orderJSON.Get("order_id").Interface()),
            Price:      conver.Float64Must(orderJSON.Get("price").MustString()),
            Amount:     conver.Float64Must(orderJSON.Get("size").MustString()),
            DealAmount: conver.Float64Must(orderJSON.Get("filled_size").MustString()),
            TradeType:  e.tradeTypeMap[orderJSON.Get("side").MustString()],
            Currency:   orderJSON.Get("instrument_id").MustString(),
            StockType:  stockType,
        })
    }
    return orders
}

// CancelOrder cancel an order
func (e *Xnodes) CancelOrder(order Order) bool {
    params := make(map[string]interface{})
    params["instrument_id"] = order.Currency
    bytesData, err := json.Marshal(params)

    jsonBody := string(bytesData)
    json, err := e.postAuthJSON("/cancel_orders/" + order.ID, jsonBody)
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "CancelOrder() error, ", err)
        return false
    }
    if result := json.Get("result").MustBool(); !result {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "CancelOrder() error, the error number is ", json.Get("code").MustInt())
        return false
    }
    e.logger.Log(constant.CANCEL, order.StockType, order.Price, order.Amount-order.DealAmount, order)
    return true
}

// getTicker get market ticker & depth
func (e *Xnodes) getTicker(stockType string, sizes ...interface{}) (ticker Ticker, err error) {
    stockType = strings.ToUpper(stockType)
    if _, ok := e.stockTypeMap[stockType]; !ok {
        err = fmt.Errorf("GetTicker() error, unrecognized stockType: %+v", stockType)
        return
    }
    size := 20
    if len(sizes) > 0 && conver.IntMust(sizes[0]) > 0 {
        size = conver.IntMust(sizes[0])
    }
    // resp, err := e.get3("/instruments/" + stockType + "/book?size=" + size)
    resp, err := e.get3(fmt.Sprintf("/instruments/%v/book?size=%v", e.stockTypeMap[stockType], size))
    if err != nil {
        err = fmt.Errorf("GetTicker() error, %+v", err)
        return
    }
    json, err := simplejson.NewJson(resp)
    if err != nil {
        err = fmt.Errorf("GetTicker() error, %+v", err)
        return
    }

    depthsJSON := json.Get("bids")
    for i := 0; i < len(depthsJSON.MustArray()); i++ {
        depthJSON := depthsJSON.GetIndex(i)
        ticker.Bids = append(ticker.Bids, OrderBook{
            Price:  depthJSON.GetIndex(0).MustFloat64(),
            Amount: depthJSON.GetIndex(1).MustFloat64(),
        })
    }
    depthsJSON = json.Get("asks")
    for i := len(depthsJSON.MustArray()); i > 0; i-- {
        depthJSON := depthsJSON.GetIndex(i - 1)
        ticker.Asks = append(ticker.Asks, OrderBook{
            Price:  depthJSON.GetIndex(0).MustFloat64(),
            Amount: depthJSON.GetIndex(1).MustFloat64(),
        })
    }
    if len(ticker.Bids) < 1 || len(ticker.Asks) < 1 {
        err = fmt.Errorf("GetTicker() error, can not get enough Bids or Asks")
        return
    }
    ticker.Buy = ticker.Bids[0].Price
    ticker.Sell = ticker.Asks[0].Price
    ticker.Mid = (ticker.Buy + ticker.Sell) / 2
    return
}

// GetTicker get market ticker & depth
func (e *Xnodes) GetTicker(stockType string, sizes ...interface{}) interface{} {
    ticker, err := e.getTicker(stockType, sizes...)
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, err)
        return false
    }
    return ticker
}

// GetRecords get candlestick data
func (e *Xnodes) GetRecords(stockType, period string, sizes ...interface{}) interface{} {
    stockType = strings.ToUpper(stockType)
    if _, ok := e.stockTypeMap[stockType]; !ok {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetRecords() error, unrecognized stockType: ", stockType)
        return false
    }
    if _, ok := e.recordsPeriodMap[period]; !ok {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetRecords() error, unrecognized period: ", period)
        return false
    }
    size := 200
    if len(sizes) > 0 && conver.IntMust(sizes[0]) > 0 {
        size = conver.IntMust(sizes[0])
    }
    resp, err := e.get3("/instruments/"+e.stockTypeMap[stockType]+"/candles?granularity=" + e.recordsPeriodMapV3[period])
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetRecords() error, ", err)
        return false
    }
    json, err := simplejson.NewJson(resp)
    if err != nil {
        e.logger.Log(constant.ERROR, "", 0.0, 0.0, "GetRecords() error, ", err)
        return false
    }
    timeLast := int64(0)
    if len(e.records[period]) > 0 {
        timeLast = e.records[period][len(e.records[period])-1].Time
    }
    recordsNew := []Record{}
    const base_format = "2006-01-02T15:04:05Z"
    for i := len(json.MustArray()); i > 0; i-- {
        recordJSON := json.GetIndex(i - 1)
        recordTimeOrigin, _ := time.Parse(base_format, recordJSON.Get("time").MustString())
        recordTime := recordTimeOrigin.UnixNano()/1e6
        if recordTime > timeLast {
            recordsNew = append([]Record{{
                Time:   recordTime,
                Open:   recordJSON.Get("open").MustFloat64(),
                High:   recordJSON.Get("high").MustFloat64(),
                Low:    recordJSON.Get("low").MustFloat64(),
                Close:  recordJSON.Get("close").MustFloat64(),
                Volume: recordJSON.Get("volume").MustFloat64(),
            }}, recordsNew...)
        } else if timeLast > 0 && recordTime == timeLast {
            e.records[period][len(e.records[period])-1] = Record{
                Time:   recordTime,
                Open:   recordJSON.Get("open").MustFloat64(),
                High:   recordJSON.Get("high").MustFloat64(),
                Low:    recordJSON.Get("low").MustFloat64(),
                Close:  recordJSON.Get("close").MustFloat64(),
                Volume: recordJSON.Get("volume").MustFloat64(),
            }
        } else {
            break
        }
    }
    e.records[period] = append(e.records[period], recordsNew...)
    if len(e.records[period]) > size {
        e.records[period] = e.records[period][len(e.records[period])-size : len(e.records[period])]
    }
    return e.records[period]
}

func (e *Xnodes) getAuthJSON(url string) (json *simplejson.Json, err error) {
    resp, err := e.get3(url)
    if err != nil {
        return
    }
    return simplejson.NewJson(resp)
}

func (e *Xnodes) postAuthJSON(url string, jsonBody string) (json *simplejson.Json, err error) {
    resp, err := e.post3(url, jsonBody)
    if err != nil {
        return
    }
    return simplejson.NewJson(resp)
}

func (e *Xnodes) get3(url string) (ret []byte, err error) {
// func get(url string) (json *simplejson.Json, err error) {
    req, err := http.NewRequest("GET", e.host + url, strings.NewReader(""))
    if err != nil {
        return
    }
    e.setHeaders(req, "GET", url, "")
    resp, err := client.Do(req)
    if resp == nil {
        err = fmt.Errorf("[GET %s] HTTP Error Info: %v", url, err)
    } else if resp.StatusCode == 200 {
        ret, _ = ioutil.ReadAll(resp.Body)
        resp.Body.Close()
    } else {
        err = fmt.Errorf("[GET %s] HTTP Status: %d, Info: %v", url, resp.StatusCode, err)
    }
    return ret, err
}

func (e *Xnodes) post3(url string, data string) (ret []byte, err error) {
    req, err := http.NewRequest("POST", e.host + url, strings.NewReader(data))
    if err != nil {
        return
    }
    e.setHeaders(req, "POST", url, data)
    resp, err := client.Do(req)
    if resp == nil {
        err = fmt.Errorf("[POST %s] HTTP Error Info: %v", url, err)
    } else if resp.StatusCode == 200 {
        ret, _ = ioutil.ReadAll(resp.Body)
        resp.Body.Close()
    } else {
        err = fmt.Errorf("[POST %s] HTTP Status: %d, Info: %v", url, resp.StatusCode, err)
    }
    return ret, err
}

func (e *Xnodes) setHeaders(req *http.Request, method string, url string, jsonBody string) {

    timestamp := IsoTime()
    preHash := timestamp + method + "/api/spot/v3" + url + jsonBody

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("OK-ACCESS-KEY", e.option.AccessKey)
    req.Header.Set("OK-ACCESS-SIGN", ComputeHmac256(preHash, e.option.SecretKey))
    req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
    req.Header.Set("OK-ACCESS-PASSPHRASE", "coffee")

}
