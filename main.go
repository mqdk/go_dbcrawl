package main

import (
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"go_dbcrawl/douban"
)

var db *gorm.DB
var log = douban.Logger

func main() {
	//initDB()
	//initGin()
	douban.RunDoubanCrawl()
}

func initGin() {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()
	r.GET("/findAll", findAll)
	r.GET("/findAll/", findAll)
	r.Run("localhost:8080")
}

func initDB() {
	var err error
	db, err = gorm.Open("mysql", "root:123456@tcp(localhost:3306)/douban?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		panic(err)
	}
	//defer db.Close()
}

func findAll(c *gin.Context) {
	var m []douban.Movie
	if err := db.Find(&m).Error; err != nil {
		c.AbortWithStatus(500)
	} else {
		log.Debugf("%+v", m)
		c.JSON(200, m)
	}
}
