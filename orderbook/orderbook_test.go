package orderbook

import (
	"fmt"
	"reflect"
	"testing"
)

func assert(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v != %+v", a, b)
	}
}

const (
	ALICE_PK = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	BOB_PK   = "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
)

func TestLimit(t *testing.T) {
	alice, _ := NewUser(ALICE_PK)

	l := NewLimit(10_000)

	buyOrderA := NewBuyOrder(5.0, alice.ID)
	buyOrderB := NewBuyOrder(8.0, alice.ID)
	buyOrderC := NewBuyOrder(10.0, alice.ID)

	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)

	l.DeleteOrder(buyOrderB)
}

func TestPlaceLimitOrder(t *testing.T) {
	ob := NewOrderbook()

	alice, _ := NewUser(ALICE_PK)

	sellOrderA := NewSellOrder(10.0, alice.ID)
	sellOrderB := NewSellOrder(5.0, alice.ID)

	ob.PlaceLimitOrder(10_000, sellOrderA)
	ob.PlaceLimitOrder(9_000, sellOrderB)

	assert(t, len(ob.asks), 2)
	assert(t, ob.Orders[sellOrderA.ID], sellOrderA)
	assert(t, ob.Orders[sellOrderB.ID], sellOrderB)
	assert(t, len(ob.Orders), 2)
}

func TestPlaceMarketOrder(t *testing.T) {
	ob := NewOrderbook()

	alice, _ := NewUser(ALICE_PK)
	bob, _ := NewUser(BOB_PK)

	sellOrder := NewSellOrder(20.0, alice.ID)
	ob.PlaceLimitOrder(10_000, sellOrder)

	buyOrder := NewBuyOrder(10.0, bob.ID)
	matches := ob.PlaceMarkerOrder(buyOrder)

	assert(t, len(matches), 1)
	assert(t, len(ob.asks), 1)
	assert(t, len(ob.Orders), 1)

	assert(t, ob.AskTotalVolume(), 10.0)
	assert(t, matches[0].Ask, sellOrder)
	assert(t, matches[0].Bid, buyOrder)
	assert(t, matches[0].SizeFilled, 10.0)
	assert(t, matches[0].Price, 10_000.0)
	assert(t, buyOrder.IsFilled(), true)
}

func TestPlaceMarketOrderMultifill(t *testing.T) {
	ob := NewOrderbook()

	alice, _ := NewUser(ALICE_PK)
	bob, _ := NewUser(BOB_PK)

	buyOrderA := NewBuyOrder(5.0, alice.ID)
	buyOrderB := NewBuyOrder(8.0, alice.ID)
	buyOrderC := NewBuyOrder(10.0, alice.ID)
	buyOrderD := NewBuyOrder(1.0, alice.ID)

	ob.PlaceLimitOrder(5_000, buyOrderD)
	ob.PlaceLimitOrder(5_000, buyOrderC)
	ob.PlaceLimitOrder(9_000, buyOrderB)
	ob.PlaceLimitOrder(10_000, buyOrderA)

	assert(t, ob.BidTotalVolume(), 24.0)

	sellOrder := NewSellOrder(20.0, bob.ID)
	matches := ob.PlaceMarkerOrder(sellOrder)
	fmt.Printf("%+v\n", matches)

	assert(t, ob.BidTotalVolume(), 4.0)
	assert(t, len(matches), 4)
	assert(t, len(ob.bids), 1)
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderbook()

	alice, _ := NewUser(ALICE_PK)

	buyOrder := NewBuyOrder(4.0, alice.ID)
	ob.PlaceLimitOrder(10_000, buyOrder)

	assert(t, len(ob.bids), 1)
	assert(t, len(ob.Orders), 1)
	assert(t, ob.BidTotalVolume(), 4.0)

	ob.CancelOrder(buyOrder)

	assert(t, ob.BidTotalVolume(), 0.0)
	assert(t, len(ob.bids), 0)
	assert(t, len(ob.Orders), 0)
}
