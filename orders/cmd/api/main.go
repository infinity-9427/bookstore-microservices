package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type orderIn struct {
	BookID   int `json:"book_id"`
	Quantity int `json:"quantity"`
}
type orderOut struct {
	ID         int     `json:"id"`
	BookID     int     `json:"book_id"`
	Quantity   int     `json:"quantity"`
	TotalPrice float64 `json:"total_price"`
}

type bookResp struct {
	ID     int     `json:"id"`
	Price  float64 `json:"price"`
	Title  string  `json:"title"`
	Author string  `json:"author"`
}

func fetchBook(base string, id int) (*bookResp, error) {
	u := fmt.Sprintf("%s/books/%d", base, id)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, errors.New("book not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("books api error: %d", resp.StatusCode)
	}
	var br bookResp
	return &br, json.NewDecoder(resp.Body).Decode(&br)
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	booksURL := os.Getenv("BOOKS_SERVICE_URL")

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil { panic(err) }
	defer pool.Close()

	_, _ = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS orders(
			id SERIAL PRIMARY KEY,
			book_id INT NOT NULL,
			quantity INT NOT NULL,
			total_price NUMERIC(10,2) NOT NULL
		)`)

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status":"ok","service":"orders"})
	})

	r.POST("/orders", func(c *gin.Context) {
		var in orderIn
		if err := c.BindJSON(&in); err != nil || in.Quantity <= 0 {
			c.JSON(400, gin.H{"error":"invalid payload"}); return
		}
		book, err := fetchBook(booksURL, in.BookID)
		if err != nil { c.JSON(400, gin.H{"error":"invalid book_id"}); return }
		total := float64(in.Quantity) * book.Price

		var id int
		err = pool.QueryRow(context.Background(),
			"INSERT INTO orders(book_id,quantity,total_price) VALUES ($1,$2,$3) RETURNING id",
			in.BookID, in.Quantity, total).Scan(&id)
		if err != nil { c.JSON(500, gin.H{"error":"db error"}); return }

		c.JSON(201, orderOut{ID:id, BookID:in.BookID, Quantity:in.Quantity, TotalPrice:total})
	})

	r.GET("/orders", func(c *gin.Context) {
		rows, _ := pool.Query(context.Background(), "SELECT id,book_id,quantity,total_price FROM orders ORDER BY id")
		defer rows.Close()
		out := []orderOut{}
		for rows.Next() {
			var o orderOut
			_ = rows.Scan(&o.ID, &o.BookID, &o.Quantity, &o.TotalPrice)
			out = append(out, o)
		}
		c.JSON(200, out)
	})

	port := os.Getenv("PORT"); if port == "" { port = "8082" }
	_ = r.Run(":" + port)
}
