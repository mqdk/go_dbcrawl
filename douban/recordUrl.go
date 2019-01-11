package douban

import (
	"fmt"
	"github.com/emirpasic/gods/sets/hashset"
	"github.com/gocolly/colly"
	"strings"
	"time"
)

type Record struct {
	ID        uint   `gorm:"primary_key"`
	Url       string `gorm:"type:varchar(255);unique;not null"`
	Crawled   uint   `gorm:"size:2;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

/**
crawl movie url
*/
func parseRecord(url string) {
	movieUrlSet := hashset.New()
	c := colly.NewCollector()
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		url := e.Attr("href")
		if !isEmpty(url) && (strings.HasPrefix(url, MOVIE_URL) || strings.HasPrefix(url, MUSIC_URL)) {
			//Logger.Infof("recordUrl: %s", url[:41])
			movieUrlSet.Add(url[:41]) // just get useful url eg:https://music.douban.com/subject/30408564/
		}
	})

	c.OnRequest(func(r *colly.Request) {
		//Logger.Infof("Record Visiting: %s", r.URL)
	})

	c.OnResponse(func(r *colly.Response) {
		Logger.Infof("Record Visited: %s", r.Request.URL)
	})

	//c.OnError(func(r *colly.Response, e error) {
	//	Logger.Errorf("Record OnError: %s", e)
	//})

	if err := c.Visit(url); err != nil {
		Logger.Errorf("Record Visit: %s error: %s", url, err)
		if err.Error() == "Forbidden" {
			recordForbidden = true
		}
	} else {
		insertRecords(movieUrlSet)
	}
}

func insertRecords(movieUrlSet *hashset.Set) {
	if movieUrlSet.Size() <= 0 {
		Logger.Debugf("movieUrlSet is empty!!!")
		return
	}
	values := movieUrlSet.Values()
	recordList := make([]interface{}, 0)
	for _, val := range values {
		record := Record{
			Url:       fmt.Sprintf("%s", val),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		recordList = append(recordList, record)
	}
	tx := db.Begin()
	err := batchInsert(tx, recordList)
	tx.Commit()
	if err != nil {
		tx.Rollback()
		Logger.Errorf("batchInsert error: %s", err)
		//!!!Deadlock found when trying to get lock; try restarting transaction
	}
}
