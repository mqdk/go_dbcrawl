package douban

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/kataras/golog"
	"strings"
	"time"
)

const (
	MOVIE_MAINURL = "https://movie.douban.com/"
	MUSIC_MAINURL = "https://music.douban.com/"
	MOVIE_URL     = "https://movie.douban.com/subject"
	MUSIC_URL     = "https://music.douban.com/subject"
	//MOVIE_REGULAR_EXP   = "https://movie.douban.com/subject/\\d{8}|\\d{7}"
	//COMMENT_REGULAR_EXP = "https://movie.douban.com/subject/\\d{8}|\\d{7}/comments"
	DOUBANID_REG_EXP = "\\d{8}|\\d{7}"
	//COMMENT_END_REG_EXP = ".*\\/comments$"
	MAXCOUNT  = 10
	WORKERNUM = 2
)

var (
	Logger           = golog.New()
	db               *gorm.DB
	recordForbidden  = false
	movieForbidden   = false
	commentForbidden = false
)

func RunDoubanCrawl() {
	initLog()
	initDB()
	beginVisit()
	//beginVisitWithMultiWorkers()

}

//single worker to do crawl
func beginVisit() {
	urlList := make([]string, 0)
	urlList = append(urlList, MOVIE_MAINURL, MUSIC_MAINURL)
	count := 0
	for {
		if recordForbidden && movieForbidden {
			Logger.Info("Forbidden!!!")
			break
		}

		count++
		if len(urlList) <= 0 && cap(urlList) <= 0 {
			//query 10 url have not been crawled yet
			db.Model(&Record{}).Where("crawled = ?", 0).Limit(10).Pluck("url", &urlList)
		}

		if len(urlList) > 0 {
			for _, url := range urlList {
				//update status crawed=1
				db.Model(&Record{}).Where("url = ?", url).Update("crawled", 1)
				parseRecord(url)

				if strings.HasPrefix(url, MOVIE_URL) {
					url = url[:41]
					parseMovie(url)
				}

				if strings.HasPrefix(url, MUSIC_URL) {
					url = url[:41]
					parseMusic(url)
				}

				friendlyToDouban()
			}
		}

		urlList = nil //clear list

		//stop loop when count above max
		if count > MAXCOUNT {
			Logger.Info("loop end,crawl finish!")
			break
		}
	}
}

//multi worker to do crawl,unsafe,may block by douban website!!!
func beginVisitWithMultiWorkers() {
	urlChan := make(chan string, 10)
	urlList := make([]string, 0)
	urlList = append(urlList, MOVIE_MAINURL)
	var stopProduce bool

	count := 0
	for {
		if recordForbidden && movieForbidden {
			Logger.Info("Forbidden!!!")
			break
		}

		count++
		if !stopProduce {
			Logger.Infof("chan size:%d", len(urlChan))
			go produce(urlList, urlChan)
		}

		consumeWorkers(urlChan)

		Logger.Info("main loop sleep...")
		time.Sleep(5 * time.Second)

		urlList = nil //clear list
		//stop loop when count above max
		if count > MAXCOUNT {
			stopProduce = true
			Logger.Infof("stop produce,chan left:%d", len(urlChan))
			if len(urlChan) == 0 {
				Logger.Info("loop end,crawl finish!")
				break
			}
		}
	}
}

//produce url
func produce(urlList []string, ch chan<- string) {
	db.Model(&Record{}).Where("crawled = ?", 0).Limit(10).Pluck("url", &urlList)
	for _, url := range urlList {
		//Logger.Infof("produce:%s", url)
		ch <- url
		db.Model(&Record{}).Where("url = ?", url).Update("crawled", 1)
	}
}

//start multi worker to consume
func consumeWorkers(ch <-chan string) {
	for i := 1; i <= WORKERNUM; i++ {
		worker := fmt.Sprintf("worker %d", i)
		go consume(ch, worker)
	}
}

//consume url
func consume(ch <-chan string, worker string) {
	for i := 0; i < len(ch); i++ {
		url := <-ch
		Logger.Infof("%s consume:%s", worker, url)
		parseRecord(url)

		if strings.HasPrefix(url, MOVIE_URL) {
			url = url[:41]
			parseMovie(url)
		}

		friendlyToDouban()
	}
}

func initDB() {
	var err error
	if db, err = gorm.Open("mysql", "root:123456@tcp(localhost:3306)/douban?charset=utf8&parseTime=True&loc=Local"); err != nil {
		panic(err)
	}
	//defer db.Close()

	db.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").AutoMigrate(&Record{}, &Movie{}, &MovieComment{}, &TagMovie{}, &TagMovieLink{}, &Album{}, &TagAlbum{}, &TagAlbumLink{}, &Song{}, &MusicComment{})
	db.Model(&Album{}).Related(&[]Song{})
}

func initLog() {
	Logger.SetTimeFormat("")
	Logger.SetLevel("debug")
}
