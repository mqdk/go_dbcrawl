package douban

import (
	"github.com/gocolly/colly"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Album struct {
	ID              uint   `gorm:"primary_key"`
	DoubanID        string `gorm:"type:varchar(8);not null;unique"`
	Name            string `gorm:"type:varchar(500);not null"`
	Singer          string `gorm:"type:varchar(255)"`
	CoverUrl        string `gorm:"type:varchar(255)"`
	Genre           string `gorm:"type:varchar(100)"`
	AlbumType       string `gorm:"type:varchar(100)"`
	ReleaseDate     string `gorm:"type:varchar(80)"`
	Publisher       string `gorm:"type:varchar(60)"`
	Summary         string `gorm:"type:varchar(8888)"`
	RatingNum       string `gorm:"type:decimal(2,1);default:0"`
	RatingPeopleNum string `gorm:"type:varchar(16)"`
	Songs           []Song `gorm:"auto_preload"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TagAlbum struct {
	ID        uint   `gorm:"primary_key"`
	TagName   string `gorm:"type:varchar(32);not null"`
	CreatedAt time.Time
}

type TagAlbumLink struct {
	AlbumID   uint `gorm:"type:varchar(16)"`
	TagID     uint `gorm:"type:varchar(16)"`
	CreatedAt time.Time
}

type Song struct {
	ID        uint   `gorm:"primary_key"`
	Name      string `gorm:"type:varchar(255)"`
	AlbumID   uint   `gorm:"type:varchar(8)"`
	CreatedAt time.Time
}

type MusicComment struct {
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

func parseMusic(url string) {
	var tagIDS []uint
	r := regexp.MustCompile(DOUBANID_REG_EXP)
	doubanId := r.FindString(url)
	album := &Album{
		DoubanID:  doubanId,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	c := colly.NewCollector()
	c.OnRequest(func(r *colly.Request) {
	})
	c.OnResponse(func(r *colly.Response) {
		Logger.Infof("music Visited: %s", r.Request.URL)
	})

	c.OnHTML("#wrapper", func(e *colly.HTMLElement) {
		name := e.ChildText("h1 span")
		album.Name = name

		img := e.ChildAttr("img", "src")
		album.CoverUrl = img

		pl := e.ChildText("#info span .pl")
		arr := strings.Split(pl, ":")
		album.Singer = trimNewline(trimString(arr[1]))

		average := e.ChildText(`strong[property="v:average"]`)
		people := e.ChildText(`span[property="v:votes"]`)
		album.RatingNum = average
		album.RatingPeopleNum = people
		summary := e.ChildText(`span[class="all hidden"]`)
		if isEmpty(summary) {
			summary = e.ChildText(`span[property="v:summary"]`) //Standby plan
		}
		album.Summary = summary

		//parse tracks
		song := &Song{
			CreatedAt: time.Now(),
		}
		songs := make([]Song, 0)
		track := e.ChildText(`div[class="track-list"]`)
		f := strings.Split(track, "\n") //diffent type,diffent parse
		//only support parse two kinds of track:
		// e.g:
		//contain ".", 1. 疯狂世界 2. 拥抱
		//or style like below
		//<div class=""> Yellow (Live In Buenos Aires) </div>
		if len(f) <= 1 {
			arrs := strings.Split(track, ".")
			for i, v := range arrs {
				if i > 0 {
					s := strconv.Itoa(i + 1)
					if strings.Contains(v, s) {
						t := strings.Replace(v, s, "", -1)
						if strings.HasSuffix(t, "0") {
							t = t[:len(t)-1]
						}
						song.Name = t
					} else {
						song.Name = v
					}
					songs = append(songs, *song)
				}
			}
		} else {
			e.ForEach(`div[class="track-list"] div div`, func(i int, e *colly.HTMLElement) {
				t := strings.TrimSpace(e.Text)
				song.Name = t
				songs = append(songs, *song)
			})
		}
		album.Songs = songs

		tagIDS = make([]uint, 0)
		e.ForEach(`div[class="tags-body"] a`, func(i int, e *colly.HTMLElement) {
			tag := &TagAlbum{
				TagName:   e.Text,
				CreatedAt: time.Now(),
			}
			tx := db.Begin()
			//insert tag,unique
			if err := tx.FirstOrCreate(tag, &TagAlbum{TagName: e.Text}).Error; err != nil {
				tx.Rollback()
			}
			tx.Commit()
			//append tag ids for relate album with tag
			tagIDS = append(tagIDS, tag.ID)
		})

	})

	c.OnHTML("div[id=info]", func(e *colly.HTMLElement) {
		contents := strings.Split(e.Text, "\n")
		for _, val := range contents {
			if strings.Contains(val, "流派") {
				arr := strings.Split(val, ":")
				album.Genre = trimString(arr[1])
			} else if strings.Contains(val, "专辑类型") {
				arr := strings.Split(val, ":")
				album.AlbumType = trimString(arr[1])
			} else if strings.Contains(val, "发行时间") {
				arr := strings.Split(val, ":")
				album.ReleaseDate = trimString(arr[1])
			} else if strings.Contains(val, "出版者") {
				arr := strings.Split(val, ":")
				album.Publisher = trimString(arr[1])
			}
		}
	})

	if err := c.Visit(url); err != nil {
		Logger.Errorf("music Visit: %s error: %s", url, err)
		if err.Error() == "Forbidden" {
			recordForbidden = true
		}
	} else {
		albumId := createAlbum(album, tagIDS)
		if albumId > 0 { //insert success
			url = url[:41]
			url += "/comments"
			parseMusicComments(url)
		}
	}
}

func createAlbum(album *Album, tagIDS []uint) uint {
	if isEmpty(album.Name) || isEmpty(album.Singer) {
		return 0
	}
	tx := db.Begin()
	//insert album info
	if err := tx.FirstOrCreate(album, &Album{DoubanID: album.DoubanID}).Error; err != nil {
		tx.Rollback()
	}
	albumId := album.ID
	albumTags := make([]interface{}, 0)
	for _, v := range tagIDS {
		albumTag := &TagAlbumLink{
			AlbumID:   albumId,
			TagID:     v,
			CreatedAt: time.Now(),
		}
		albumTags = append(albumTags, albumTag)
	}
	//relate album and tag
	if err := batchInsert(tx, albumTags); err != nil {
		tx.Rollback()
	}
	tx.Commit()
	return albumId
}

func parseMusicComments(url string) {
	r := regexp.MustCompile(DOUBANID_REG_EXP)
	doubanId := r.FindString(url)
	comments := make([]interface{}, 0)

	c := colly.NewCollector()
	c.OnRequest(func(r *colly.Request) {
	})
	c.OnResponse(func(r *colly.Response) {
		Logger.Infof("music comment Visited: %s", r.Request.URL)
	})

	c.OnHTML("#comments", func(e *colly.HTMLElement) {
		var star int
		e.ForEach(`li[class="comment-item"]`, func(i int, e *colly.HTMLElement) {
			comment := &MusicComment{
				DoubanID:  doubanId,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			img := e.ChildAttr("img", "src")
			comment.Avatar = img
			title := e.ChildAttr("a", "title")
			comment.Author = title
			vote := e.ChildText(`span[class="vote-count"]`)
			comment.VoteNum = vote
			info := e.ChildText(`span[class="short"]`)
			comment.Info = info

			date := e.ChildText(`div[class="comment"] span[class="comment-info"] span`)
			comment.CommentDate = date

			commentInfos := e.ChildAttrs(`div[class="comment"] span[class="comment-info"] span`, "title")
			for _, v := range commentInfos {
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
			}
			comments = append(comments, comment)
		})

	})

	if err := c.Visit(url); err != nil {
		Logger.Errorf("music comment to Visit:%s error:%s", url, err)
		if err.Error() == "Forbidden" {
			commentForbidden = true
		}
	} else {
		createMusicComments(comments)
		//Logger.Debugf("%+q", comments)
	}
}

func createMusicComments(comments []interface{}) {
	tx := db.Begin()
	if err := batchInsert(tx, comments); err != nil {
		tx.Rollback()
	}
	tx.Commit()
}
