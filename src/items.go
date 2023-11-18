package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
)

type Item struct {
	Id          string
	Name        string `form:"name"`
	Description string `form:"description"`
	ImageUrl    string `form:"imageUrl"`
	Price       string `form:"price"`
}

func CreateItem(ctx *gin.Context) {
	if authenticated := authorize(ctx); !authenticated {
		ctx.String(http.StatusForbidden, "")
		return
	}
	var item Item
	if err := ctx.ShouldBind(&item); err != nil {
		ctx.String(http.StatusBadRequest, "invalid post body")
		return
	}
	itemId, err := ksuid.NewRandom()
	if err != nil {
		log.Printf("failed to generate ksuid - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	item.Id = itemId.String()

	//_, err = db.Exec(`INSERT into ITEMS(id, name, description, imageurl, price) values (` + fmt.Sprintf(`'%s','%s','%s','%s','%s'`, item.Id, item.Name, item.Description, item.ImageUrl, item.Price) + `)`)
	_, err = db.Exec(`INSERT into ITEMS(id, name, description, imageurl, price) values ($1, $2, $3, $4, $5)`, item.Id, item.Name, item.Description, item.ImageUrl, item.Price)
	if err != nil {
		log.Printf("failed to store item - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	bytes, err := json.Marshal(item)
	if err != nil {
		log.Printf("failed to marshal - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data(http.StatusCreated, "application/json", bytes)
}

func DeleteItem(ctx *gin.Context) {
	if authenticated := authorize(ctx); !authenticated {
		ctx.String(http.StatusForbidden, "")
		return
	}
	itemId := ctx.Param("item")

	_, err := db.Exec("DELETE FROM ITEMS WHERE id = '" + itemId + "'")
	if err != nil {
		log.Printf("failed to lookup item - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.String(http.StatusAccepted, "deleted item %s", itemId)
}

func ListItems(ctx *gin.Context) {
	rows, err := db.Query("SELECT * FROM ITEMS")
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.Data(http.StatusOK, "application/json", []byte("[]"))
		} else {
			log.Printf("failed to lookup items - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		return
	}

	items := make([]Item, 0)

	for rows.Next() {
		item := Item{}
		err := rows.Scan(&item.Id, &item.Name, &item.Description, &item.ImageUrl, &item.Price)
		if err != nil {
			log.Printf("failed to extract row info - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, item)
	}

	bytes, err := json.Marshal(items)
	if err != nil {
		log.Printf("failed to marshal - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data(http.StatusOK, "application/json", bytes)
}

func DescribeItem(ctx *gin.Context) {
	itemId := ctx.Param("item")

	item := Item{
		Id: itemId,
	}

	err := db.QueryRow("SELECT * FROM ITEMS WHERE id = '"+itemId+"'").Scan(&item.Id, &item.Name, &item.Description, &item.ImageUrl, &item.Price)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.String(http.StatusNotFound, ":(")
		} else {
			log.Printf("failed to lookup item - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		return
	}

	bytes, err := json.Marshal(item)
	if err != nil {
		log.Printf("failed to marshal - %s", err)
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Data(http.StatusAccepted, "application/json", bytes)
}

type purchaseInfo struct {
	Payment        Card    `json:"payment"`
	MailingAddress Address `json:"mailingAddress"`
}

func BuyItem(ctx *gin.Context) {
	itemId := ctx.Param("item")

	item := Item{
		Id: itemId,
	}

	err := db.QueryRow("SELECT * FROM ITEMS WHERE id = '"+itemId+"'").Scan(&item.Id, &item.Name, &item.Description, &item.ImageUrl, &item.Price)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.String(http.StatusNotFound, ":(")
		} else {
			log.Printf("failed to lookup item - %s", err)
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		return
	}

	var purchaseInfo purchaseInfo
	if err := ctx.ShouldBindJSON(&purchaseInfo); err != nil {
		ctx.String(http.StatusBadRequest, "invalid post body", err.Error())
		return
	}

	if err := CreateOrder(Order{
		Item:           item,
		Date:           time.Now(),
		Payment:        purchaseInfo.Payment,
		MailingAddress: purchaseInfo.MailingAddress,
	}); err != nil {
		ctx.String(http.StatusPaymentRequired, "failed to purchase item %s", itemId)
	}
	ctx.String(http.StatusAccepted, "purchased item %s", itemId)
}
