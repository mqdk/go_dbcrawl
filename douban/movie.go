package douban

import (
	"github.com/gocolly/colly"
	"regexp"
	"strings"
	"time"
)

type Movie struct {
	ID          uint   `gorm:"primary_key"`
	DoubanID    string `gorm:"type:varchar(8);not null;unique"`
	Name        string `gorm:"type:varchar(100);not null"`
	Director    string `gorm:"type:varchar(255)"`
	Scenarist   string `gorm:"type:varchar(100)"`
	Actors      string `gorm:"type:varchar(500)"`
	Type        string `gorm:"type:varchar(80)"`
	Country     string `gorm:"type:varchar(80)"`
	Language    string `gorm:"type:varchar(60)"`
	ReleaseDate string `gorm:"type:varchar(80)"`
	Runtime     string `gorm:"type:varchar(30)"`
	RatingNum   string `gorm:"type:decimal(2,1);default:0"`
	//Tag        string `gorm:"type:varchar(100)"`
	//Tags      []Tag `gorm:"many2many:movie_tag"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TagMovie struct {
	ID        uint   `gorm:"primary_key"`
	TagName   string `gorm:"type:varchar(32);not null"`
	CreatedAt time.Time
}

type TagMovieLink struct {
	MovieID   uint `gorm:"type:varchar(16)"`
	TagID     uint `gorm:"type:varchar(16)"`
	CreatedAt time.Time
}

type MovieComment struct {
	ID          uint   `gorm:"primary_key"`
	DoubanID    string `gorm:"type:varchar(8);not null"`
	Info        string `gorm:"type:varchar(355);unique;not null"`
	Author      string `gorm:"type:varchar(50)"`
	Avatar      string `gorm:"type:varchar(128)"`
	VoteNum     string `gorm:"type:varchar(10);default:0"`
	CommentDate string `gorm:"type:varchar(128)"`
	Star        int    `gorm:"size:1;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

/**
crawl movie info
*/
func parseMovie(url string) {
	var tagIDS []uint
	r := regexp.MustCompile(DOUBANID_REG_EXP)
	doubanId := r.FindString(url)
	movie := &Movie{
		DoubanID:  doubanId,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	c := colly.NewCollector()
	c.OnRequest(func(r *colly.Request) {
		//Logger.Infof("movie Visiting: %s", r.URL)
	})
	c.OnResponse(func(r *colly.Response) {
		Logger.Infof("movie Visited: %s", r.Request.URL)
	})

	c.OnHTML("#content", func(e *colly.HTMLElement) {
		//text := e.ChildText(`span[property="v:itemreviewed"]`)
		//Logger.Infof("itemreviewed:%s", text)
		var name string
		average := e.ChildText(`strong[property="v:average"]`)
		movie.RatingNum = average
		//img := e.ChildAttr("img", "src")
		//Logger.Infof("img:%s", img)

		e.ForEach("h1 span", func(i int, e *colly.HTMLElement) {
			name += e.Text
		})
		movie.Name = name

		tagIDS = make([]uint, 0)
		e.ForEach(`div[class="tags-body"] a`, func(i int, e *colly.HTMLElement) {
			tag := &TagMovie{
				TagName:   e.Text,
				CreatedAt: time.Now(),
			}
			tx := db.Begin()
			//insert tag,unique
			if err := tx.FirstOrCreate(tag, &TagMovie{TagName: e.Text}).Error; err != nil {
				tx.Rollback()
			}
			tx.Commit()
			//append tag ids for relate movie with tag
			tagIDS = append(tagIDS, tag.ID)
		})
	})

	c.OnHTML("#info span", func(e *colly.HTMLElement) {
		key := e.ChildText(".pl")
		if key == "导演" {
			text := e.ChildText(".attrs")
			movie.Director = trimString(text)
		} else if key == "编剧" {
			text := e.ChildText(".attrs")
			movie.Scenarist = trimString(text)
		} else if key == "主演" {
			text := e.ChildText(".attrs")
			movie.Actors = trimString(text)
		}

	})

	c.OnHTML("div[id=info]", func(e *colly.HTMLElement) {
		var genre string
		var releaseDate string
		e.ForEach(`span[property="v:genre"]`, func(i int, e *colly.HTMLElement) {
			genre = strings.Join([]string{e.Text, genre}, "/")
		})
		movie.Type = genre

		e.ForEach(`span[property="v:initialReleaseDate"]`, func(i int, e *colly.HTMLElement) {
			releaseDate = strings.Join([]string{e.Text, releaseDate}, "/")
		})
		movie.ReleaseDate = releaseDate

		runtime := e.ChildText(`span[property="v:runtime"]`)
		movie.Runtime = runtime

		contents := strings.Split(e.Text, "\n")
		for _, val := range contents {
			if strings.Contains(val, "制片国家/地区") {
				arr := strings.Split(val, ":")
				movie.Country = trimString(arr[1])
			} else if strings.Contains(val, "语言") {
				arr := strings.Split(val, ":")
				movie.Language = trimString(arr[1])
			}
		}
	})

	if err := c.Visit(url); err != nil {
		Logger.Errorf("movie to Visit:%s error:%s", url, err)
		if err.Error() == "Forbidden" {
			movieForbidden = true
		}
	} else {
		//Logger.Debugf("%+v", movie)
		movieId := createMovie(movie, tagIDS)
		if movieId > 0 { //insert success
			url = url[:41]
			url += "/comments"
			parseMovieComment(url)
		}
	}

}

func createMovie(movie *Movie, tagIDS []uint) uint {
	if isEmpty(movie.Name) || (isEmpty(movie.Director) && isEmpty(movie.Scenarist)) {
		return 0
	}
	tx := db.Begin()
	//insert movie info
	if err := tx.FirstOrCreate(movie, &Movie{DoubanID: movie.DoubanID}).Error; err != nil {
		tx.Rollback()
	}
	movieId := movie.ID
	movieTags := make([]interface{}, 0)
	for _, v := range tagIDS {
		//Logger.Infof("tagIDS:%d", v)
		movieTag := &TagMovieLink{
			MovieID:   movieId,
			TagID:     v,
			CreatedAt: time.Now(),
		}
		movieTags = append(movieTags, movieTag)
	}
	//relate movie and tag
	if err := batchInsert(tx, movieTags); err != nil {
		tx.Rollback()
	}
	tx.Commit()
	return movieId
}

func parseMovieComment(url string) {
	r := regexp.MustCompile(DOUBANID_REG_EXP)
	doubanId := r.FindString(url)
	comments := make([]interface{}, 0)

	c := colly.NewCollector()
	c.OnRequest(func(r *colly.Request) {
		//Logger.Infof("movie Visiting: %s", r.URL)
	})
	c.OnResponse(func(r *colly.Response) {
		Logger.Infof("comment Visited: %s", r.Request.URL)
	})

	c.OnHTML("#comments", func(e *colly.HTMLElement) {
		var star int
		e.ForEach(`div[class="comment-item"]`, func(i int, e *colly.HTMLElement) {
			comment := &MovieComment{
				DoubanID:  doubanId,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			img := e.ChildAttr("img", "src")
			comment.Avatar = img
			title := e.ChildAttr("a", "title")
			comment.Author = title
			vote := e.ChildText(`span[class="votes"]`)
			comment.VoteNum = vote
			info := e.ChildText(`span[class="short"]`)
			comment.Info = info

			commentInfos := e.ChildAttrs(`div[class="comment"] span[class="comment-info"] span`, "title")
			for k, v := range commentInfos {
				if k == 0 {
					if !isEmpty(v) {
						switch v {
						case "力荐":
							star = 5
							break
						case "推荐":
							star = 4
							break
						case "还行":
							star = 3
							break
						case "较差":
							star = 2
							break
						case "很差":
							star = 1
							break
						default:
							star = 0
						}
					}
					comment.Star = star
				} else if k == 1 {
					comment.CommentDate = v
				}
			}
			comments = append(comments, comment)
		})

	})

	if err := c.Visit(url); err != nil {
		Logger.Errorf("comment to Visit:%s error:%s", url, err)
		if err.Error() == "Forbidden" {
			commentForbidden = true
		}
	} else {
		createComments(comments)
	}
}

func createComments(comments []interface{}) {
	tx := db.Begin()
	if err := batchInsert(tx, comments); err != nil {
		tx.Rollback()
	}
	tx.Commit()
}
