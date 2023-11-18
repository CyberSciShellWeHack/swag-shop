package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	log.Println("Connecting to db...")
	var err error
	db, err = setupDatabase()
	if err != nil {
		log.Fatalf("failed to setup db - %s", err)
	}
	defer db.Close()
	log.Println("Connected to db")

	go genAuthCheck()

	log.Println("Setting up shop apis...")
	gin.SetMode(gin.ReleaseMode) // Get rid of those pesky debug logs
	router := gin.Default()
	// Admin only requests
	router.POST("/shop/items", CreateItem)
	router.DELETE("/shop/items/:item", DeleteItem)
	router.GET("/shop/orders", ListOrders)

	// Anyone can access requests
	router.GET("/shop/items", ListItems)
	router.GET("/shop/items/:item", DescribeItem)
	router.POST("/shop/items/:item", BuyItem)
	router.GET("/auth", GetAuthMessage)

	log.Printf("Listening on :443")

	if err := http.ListenAndServeTLS(":443", "server.crt", "server.key", router); err != nil {
		log.Fatalf("failed to create server - %w", err)
	}
}

func setupDatabase() (*sql.DB, error) {
	// Now connect to our new db
	db, err := sql.Open("sqlite3", "swag.db")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database - %s", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping db - %s", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS ITEMS"); err != nil {
		log.Fatalf("failed to delete table - %s", err)
	}
	if _, err := db.Exec("CREATE TABLE ITEMS(id VARCHAR(32) NOT NULL, name VARCHAR(64) NOT NULL, description VARCHAR(256) NULL, imageurl VARCHAR(1024) NULL, price VARCHAR(10) NOT NULL)"); err != nil {
		return db, fmt.Errorf("failed to create items table - %s", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS ORDERS"); err != nil {
		log.Fatalf("failed to delete table - %s", err)
	}
	if _, err := db.Exec("CREATE TABLE ORDERS(id VARCHAR(128) NOT NULL, item VARCHAR(2048) NOT NULL, date VARCHAR(1024) NOT NULL, payment VARCHAR(3056) NOT NULL, mailingaddress VARCHAR(2048) NOT NULL)"); err != nil {
		return db, fmt.Errorf("failed to create items table - %s", err)
	}
	return db, nil
}
