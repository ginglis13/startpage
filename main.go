package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"html/template"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mmcdole/gofeed"
)

const (
	// lambdaStartpageLocation is where Lambda stores the final page. Lambda provides /tmp as a workspace.
	lambdaStartpageLocation = "/tmp/startpage.html"
	// templateFileLocation is the location of the template file in the Lambda function's environment.
	templateFileLocation = "startpage-template.html"
	// feedFileLocation is the location of the feeds config in the Lambda function's environment.
	feedFileLocation = "feeds.txt"
	// interval defines from how long in the past to include posts.
	interval = 48 * time.Hour
)

type Feed struct {
	url string
}

type Post struct {
	Source, Title, Url string
}

type StartPageData struct {
	Posts []Post
}

// readFeedConfig reads rss / atom feeds from a config text file
func readFeedConfig() []Feed {
	// Maintain feeds in simple text file
	feedFile, err := os.Open(feedFileLocation)
	if err != nil {
		log.Fatalf("[readFeedConfig] could not open feed config: %v\n", err)
	}

	scanner := bufio.NewScanner(feedFile)
	scanner.Split(bufio.ScanLines)

	var feeds []Feed
	for scanner.Scan() {
		feeds = append(feeds, Feed{scanner.Text()})
	}

	return feeds
}

// parseFeedForNewPosts finds posts made within the pastTime interval.
func parseFeedForNewPosts(url string) (*Post, error) {
	// set 1s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, err
	}

	// Loop over items. If item has PublishedParsed (*time.Time) > pastTime, return it.
	pastTime := time.Now().UTC().Add(-1 * interval)
	items := feed.Items
	for _, item := range items {
		if item.PublishedParsed.UTC().After(pastTime) {
			// return immediately, assume that only 1 post has been made in last day
			// since these are pretty much all independent blogs
			return &Post{Title: item.Title, Url: item.Link, Source: feed.Title}, nil
		}
	}
	return nil, errors.New("no new posts")
}

// fetchFeed takes a feed endpoint and populates postChan with new Posts.
func fetchFeed(feed Feed, postChan chan Post, wg *sync.WaitGroup) {
	defer wg.Done()
	post, err := parseFeedForNewPosts(feed.url)
	if err != nil {
		log.Printf("[fetchFeed] No updates found for %v: %v\n", feed.url, err.Error())
		return
	}
	postChan <- *post
}

// generateStartpage reads from the channel of new posts and returns the populated
// startpage html template as an io.Reader to be used as the PutObject request Body.
func generateStartpage(postChan <-chan Post) io.Reader {
	startPageData := StartPageData{}

	for post := range postChan {
		startPageData.Posts = append(startPageData.Posts, post)
	}

	// Included in zip uploaded to Lambda. Could also be defined as string in this
	// file but easier to edit and track w vcs if decoupled
	tmpl := template.Must(template.ParseFiles(templateFileLocation))

	f, err := os.Create(lambdaStartpageLocation)
	if err != nil {
		log.Fatalf("[generateStartPage] could not create template file: %v\n", err)
	}
	err = tmpl.Execute(f, startPageData)
	if err != nil {
		log.Fatalf("[generateStartPage] could not execute template file: %v\n", err)
	}

	data, err := os.ReadFile(lambdaStartpageLocation)
	if err != nil {
		log.Fatalf("[generateStartPage] could not read startpage: %v\n", err)
	}
	body := bytes.NewReader(data)

	return body
}

func putStartpage(body io.Reader) {
	// Create S3 Client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	cfg.Region = os.Getenv("S3_BUCKET_REGION")
	client := s3.NewFromConfig(cfg)

	s3Bucket := os.Getenv("S3_BUCKET")
	s3FileKey := os.Getenv("S3_FILE_KEY")

	// Upload file to s3 as start/index.html
	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(s3FileKey),
		Body:        body,
		ContentType: aws.String("text/html"),
	}
	_, err = client.PutObject(context.Background(), putObjectInput)
	if err != nil {
		log.Fatal("[putStartPage] Error uploading object to S3: ", err)
	}

	log.Println("[putStartPage] successfully updated startpage")
}

// LambdaMainWrapper wraps main since lambda expects entrypoint to call lambda.Start(func())
func LambdaMainWrapper() {
	feeds := readFeedConfig()

	var wg sync.WaitGroup
	postChan := make(chan Post, len(feeds)) // max one post per feed per day

	for _, feed := range feeds {
		wg.Add(1)
		go fetchFeed(feed, postChan, &wg)
	}

	wg.Wait()
	close(postChan)

	bodyToPut := generateStartpage(postChan)
	putStartpage(bodyToPut)
}

func main() {
	lambda.Start(LambdaMainWrapper)
}
