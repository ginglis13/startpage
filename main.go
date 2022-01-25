package main

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/mmcdole/gofeed"
)

type Feed struct {
	site, url string
}

type Post struct {
	Source, Title, Url string
}

type StartPageData struct {
	Posts []Post
}

// parseFeedForNewPosts determines if there is a new post in the last day
func parseFeedForNewPosts(url string) (*Post, error) {
	// set 5s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, err
	}

	// Loop over items. If item has PublishedParsed (*time.Time) > yesterday, return it
	items := feed.Items
	yesterday := time.Now().Add(-240000 * time.Hour)
	for _, item := range items {
		if item.PublishedParsed.After(yesterday) {
			// return immediately, assume that only 1 post has been made in last day
			// since these are pretty much all independent blogs
			return &Post{Title: item.Title, Url: item.Link}, nil
		}
	}
	return nil, errors.New("no new posts")
}

// fetchFeed takes a feed endpoint and populates postChan with new Posts
func fetchFeed(feed Feed, postChan chan Post, wg *sync.WaitGroup) {
	defer wg.Done()
	post, err := parseFeedForNewPosts(feed.url)
	if err != nil {
		log.Printf("No updates to %v: %v", feed.site, err.Error())
		return
	}
	post.Source = feed.site
	postChan <- *post
}

// generateStartpage reads from the channel of new posts and writes them to the startpage html template
func generateStartpage(postChan <-chan Post) {
	startPageData := StartPageData{}

	for post := range postChan {
		startPageData.Posts = append(startPageData.Posts, post)
	}

	tmpl := template.Must(template.ParseFiles("startpage-template.html"))
	f, err := os.Create("./startpage.html")
	if err != nil {
		log.Fatal("could not create template file")
	}
	tmpl.Execute(f, startPageData)
}

func main() {
	feeds := [2]Feed{
		{"Drew DeVault's Blog", "https://drewdevault.com/blog/index.xml"},
		{"Dave Cheney's Blog", "https://dave.cheney.net/atom"},
	}

	var wg sync.WaitGroup
	postChan := make(chan Post, len(feeds))

	for _, feed := range feeds {
		wg.Add(1)
		go fetchFeed(feed, postChan, &wg)
	}

	wg.Wait()
	close(postChan)

	generateStartpage(postChan)
}
