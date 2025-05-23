package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vyorkin/crypto-exchange/orderbook"
)

type Market string

const MarketETH = "ETH"

type Exchange struct {
	orderbooks map[Market]*orderbook.Orderbook
}

func NewExchange() *Exchange {
	orderbooks := make(map[Market]*orderbook.Orderbook)
	orderbooks[MarketETH] = orderbook.NewOrderbook()

	return &Exchange{
		orderbooks,
	}
}

type OrderType string

const (
	MarketOrder OrderType = "MARKET"
	LimitOrder  OrderType = "LIMIT"
)

type PlaceOrderRequest struct {
	Type   OrderType
	Bid    bool
	Size   float64
	Price  float64
	Market Market
}

type Order struct {
	ID        uuid.UUID `json:"id"`
	Price     float64   `json:"price"`
	Size      float64   `json:"size"`
	Bid       bool      `json:"bid"`
	Timestamp int64     `json:"timestamp"`
}

type MatchedOrder struct {
	ID    uuid.UUID `json:"id"`
	Price float64   `json:"price"`
	Size  float64   `json:"size"`
}

type OrderbookData struct {
	TotalBidVolume float64  `json:"total_bid_volume"`
	TotalAskVolume float64  `json:"total_ask_volume"`
	Asks           []*Order `json:"asks"`
	Bids           []*Order `json:"bids"`
}

func (ex *Exchange) handleGetBook(c *gin.Context) {
	market := Market(c.Param("market"))

	ob, exists := ex.orderbooks[market]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "Market not found",
		})
	}

	orderbookData := OrderbookData{
		Asks:           []*Order{},
		Bids:           []*Order{},
		TotalBidVolume: ob.BidTotalVolume(),
		TotalAskVolume: ob.AskTotalVolume(),
	}

	for _, askLimit := range ob.Asks() {
		for _, askOrder := range askLimit.Orders {
			order := Order{
				ID:        askOrder.ID,
				Price:     askLimit.Price,
				Size:      askOrder.Size,
				Bid:       askOrder.Bid,
				Timestamp: askOrder.Timestamp,
			}
			orderbookData.Asks = append(orderbookData.Asks, &order)
		}
	}

	for _, bidLimit := range ob.Bids() {
		for _, bidOrder := range bidLimit.Orders {
			order := Order{
				ID:        bidOrder.ID,
				Price:     bidLimit.Price,
				Size:      bidOrder.Size,
				Bid:       bidOrder.Bid,
				Timestamp: bidOrder.Timestamp,
			}
			orderbookData.Bids = append(orderbookData.Bids, &order)
		}
	}

	c.JSON(http.StatusOK, orderbookData)
}

func (ex *Exchange) handlePlaceOrder(c *gin.Context) {
	var orderData PlaceOrderRequest

	if err := c.BindJSON(&orderData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	market := Market(orderData.Market)
	ob := ex.orderbooks[market]

	order := orderbook.NewOrder(orderData.Bid, orderData.Size)
	if orderData.Type == LimitOrder {
		ob.PlaceLimitOrder(orderData.Price, order)
		c.JSON(http.StatusCreated, orderData)
		return
	}

	if orderData.Type == MarketOrder {
		matches := ob.PlaceMarkerOrder(order)

		matchedOrders := make([]*MatchedOrder, len(matches))
		for i, match := range matches {
			id := matches[i].Bid.ID
			if order.Bid {
				id = matches[i].Ask.ID
			}
			matchedOrders[i] = &MatchedOrder{
				ID:    id,
				Size:  match.SizeFilled,
				Price: match.Price,
			}
		}

		c.JSON(http.StatusCreated, gin.H{
			"matches": matchedOrders,
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"status": "Invalid order type",
	})
}

func (ex *Exchange) handleCancelOrder(c *gin.Context) {
	market := Market(c.Param("market"))

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "invalid order id",
		})
		return
	}

	ob, exists := ex.orderbooks[market]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "Market not found",
		})
	}

	order, exists := ob.Orders[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "Order not found",
		})
	}
	ob.CancelOrder(order)

	c.JSON(http.StatusOK, order)
}

func main() {
	ex := NewExchange()

	router := gin.Default()

	router.GET("/book/:market", ex.handleGetBook)
	router.POST("/order", ex.handlePlaceOrder)
	router.DELETE("/order/:market/:id", ex.handleCancelOrder)

	router.Run("localhost:3000")
}
