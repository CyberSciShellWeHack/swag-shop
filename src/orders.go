package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

type Order struct {
	Id             string
	Item           Item
	Date           time.Time
	Payment        Card
	MailingAddress Address
}

type Address struct {
	Province   string `json:"province"`
	City       string `json:"city"`
	Street     string `json:"street"`
	PostalCode string `json:"postalCode"`
}

type Card struct {
	Number       string  `json:"number"`
	ExpiryDate   string  `json:"expiryDate"`
	SecurityCode string  `json:"securityCode"`
	Address      Address `json:"address"`
}

func (c Card) Validate() bool {
	if len(c.Number) < 10 || len(c.Number) > 20 {
		return false
	}
	if len(c.SecurityCode) < 3 || len(c.SecurityCode) > 5 {
		return false
	}
	if c.ExpiryDate == "" {
		return false
	}
	if (c.Address == Address{}) {
		return false
	}
	return true
}

func CreateOrder(order Order) error {

	orderId, err := ksuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to generate order id - %w", err)
	}
	order.Id = orderId.String()

	itemJson, err := json.Marshal(order.Item)
	if err != nil {
		return fmt.Errorf("failed to marshal item - %w", err)
	}
	dateJson, err := json.Marshal(order.Date)
	if err != nil {
		return fmt.Errorf("failed to marshal date - %w", err)
	}
	paymentJson, err := json.Marshal(order.Payment)
	if err != nil {
		return fmt.Errorf("failed to marshal payment - %w", err)
	}
	mailAdrJson, err := json.Marshal(order.MailingAddress)
	if err != nil {
		return fmt.Errorf("failed to marshal mailing address - %w", err)
	}

	_, err = db.Exec(`INSERT into ORDERS(id, item, date, payment, mailingaddress) values (` + fmt.Sprintf(`'%s','%s','%s','%s','%s'`, order.Id, string(itemJson), string(dateJson), string(paymentJson), string(mailAdrJson)) + `)`)
	if err != nil {
		return fmt.Errorf("failed to create order - %w", err)
	}

	return nil
}

func ListOrders(ctx *gin.Context) {
	if authenticated := authorize(ctx); !authenticated {
		ctx.String(http.StatusForbidden, "")
		return
	}

	rows, err := db.Query("SELECT * FROM ORDERS")
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.Data(http.StatusOK, "application/json", []byte("[]"))
		} else {
			log.Printf("failed to lookup orders - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		return
	}

	orders := make([]Order, 0)

	for rows.Next() {
		order := Order{}

		var itemJson string
		var dateJson string
		var paymentJson string
		var mailingJson string
		err := rows.Scan(&order.Id, &itemJson, &dateJson, &paymentJson, &mailingJson)
		if err != nil {
			log.Printf("failed to extract row info - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		err = json.Unmarshal([]byte(itemJson), &order.Item)
		if err != nil {
			log.Printf("failed to unmarshal item - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		err = json.Unmarshal([]byte(dateJson), &order.Date)
		if err != nil {
			log.Printf("failed to unmarshal date - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		err = json.Unmarshal([]byte(paymentJson), &order.Payment)
		if err != nil {
			log.Printf("failed to unmarshal payment - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		err = json.Unmarshal([]byte(mailingJson), &order.MailingAddress)
		if err != nil {
			log.Printf("failed to unmarshal item - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		orders = append(orders, order)
	}

	bytes, err := json.Marshal(orders)
	if err != nil {
		log.Printf("failed to marshal - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data(http.StatusOK, "application/json", bytes)
}
