package douban

import (
	"github.com/gocolly/colly"
)

func parseMusic(url string) {

	c := colly.NewCollector()
	c.OnRequest(func(r *colly.Request) {
		//Logger.Infof("movie Visiting: %s", r.URL)
	})
	c.OnResponse(func(r *colly.Response) {
		Logger.Infof("music Visited: %s", r.Request.URL)
	})

	c.OnHTML("#content", func(e *colly.HTMLElement) {

	})

	if err := c.Visit(url); err != nil {
		Logger.Errorf("music Visit: %s error: %s", url, err)
		if err.Error() == "Forbidden" {
			recordForbidden = true
		}
	} else {
	}
}
