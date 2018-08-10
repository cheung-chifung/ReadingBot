package main

import (
	"log"
	"net/url"
	"os"
	"testing"

	"github.com/jinzhu/gorm"
)

func Test_createPostManager(t *testing.T) {
	db, err := gorm.Open("sqlite3", "test.db")
	if err != nil {
		log.Println(err)
		panic("failed to connect database")
	}
	// db.CreateTable(&Post{})
	db.AutoMigrate(&Post{})

	defer func() {
		db.Close()
		_ = os.Remove("test.db")
	}()

	pm := NewPostManager(db)
	s := pm.GetState(1)
	log.Println(s)

	u, err := url.Parse("https://google.com?q=12345")
	if err != nil {
		panic("fail to parse url")
	}

	ps, err := pm.LoadOrStorePostState(1, u)
	if err != nil {
		log.Fatal("fail to load url:", err)
	}
	log.Println(ps)
}
