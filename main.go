package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vyorkin/crypto-exchange/orderbook"
)

type Market string

const MarketETH = "ETH"

type Exchange struct {
	PrivateKey *ecdsa.PrivateKey
	orderbooks map[Market]*orderbook.Orderbook
}

func NewExchange(privateKeyStr string) (*Exchange, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		return nil, err
	}

	orderbooks := make(map[Market]*orderbook.Orderbook)
	orderbooks[MarketETH] = orderbook.NewOrderbook()

	return &Exchange{
		PrivateKey: privateKey,
		orderbooks: orderbooks,
	}, nil
}

type User struct {
	ID         uuid.UUID
	PrivateKey *ecdsa.PrivateKey
}

func NewUser(privateKeyStr string) (*User, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:         uuid.New(),
		PrivateKey: privateKey,
	}, nil
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

const exchangePrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func main() {
	exchange, err := NewExchange(exchangePrivateKey)
	if err != nil {
		log.Fatal(err)
	}

	router := gin.Default()

	router.GET("/book/:market", exchange.handleGetBook)
	router.POST("/order", exchange.handlePlaceOrder)
	router.DELETE("/order/:market/:id", exchange.handleCancelOrder)

	client, err := ethclient.Dial("http://localhost:8545")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	address := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")

	balance, err := client.BalanceAt(ctx, address, nil)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("balance: %v", balance)

	publicKey := exchange.PrivateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}
	value := big.NewInt(1000000000000000000) // in wei (1 eth)

	gasLimit := uint64(21000) // in units
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, nil)
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), exchange.PrivateKey)
	if err != nil {
		log.Fatal(err)
	}
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("tx sent: %s", signedTx.Hash().Hex()) // tx sent: 0x77006fcb3938f648e2cc65bafd27dec30b9bfbe9df41f78498b9c8b7322a249e

	router.Run("localhost:3000")
}
