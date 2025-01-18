package scrape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mempirate/scholar/document"
)

const ENDPOINT = "https://api.x.com/2/"

type TweetJSON struct {
	Data struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		NoteTweet struct {
			Text string `json:"text"`
		} `json:"note_tweet"`
		// time.RFC3339 (ISO 8601)
		CreatedAt        string `json:"created_at"`
		AuthorID         string `json:"author_id"`
		ReferencedTweets []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"referenced_tweets"`
	} `json:"data"`
	Includes struct {
		Users []struct {
			Username string `json:"username"`
		} `json:"users"`
		Tweets []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			NoteTweet struct {
				Text string `json:"text"`
			} `json:"note_tweet"`
			// time.RFC3339 (ISO 8601)
			CreatedAt string `json:"created_at"`
			AuthorID  string `json:"author_id"`
		} `json:"tweets"`
	} `json:"includes"`
}

type TweetData struct {
	Tweet
	QuotedTweets  []Tweet `json:"quoted_tweets"`
	RepliedTweets []Tweet `json:"replied_tweets"`
}

type Tweet struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Username  string `json:"username,omitempty"`
	AuthorID  string `json:"author_id,omitempty"`
	Text      string `json:"text"`
}

// GetTweet returns a Tweet by ID, in document format. It also returns referenced tweets with depth 1, where referenced
// tweets are tweets that are quoted or replied to.
func GetTweet(id string) (*document.Document, error) {
	// Get tweet by ID
	client := &http.Client{}
	req, err := http.NewRequest("GET", ENDPOINT+"tweets/"+id, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("Authorization", "Bearer "+os.Getenv("X_BEARER_TOKEN"))

	q := req.URL.Query()

	// Get referenced tweets (depth = 1)
	q.Set("tweet.fields", "note_tweet,created_at,author_id,referenced_tweets")
	// Expand author and referenced tweets
	q.Set("expansions", "author_id,referenced_tweets.id")

	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tweet: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var tweet TweetJSON
	if err := json.Unmarshal(body, &tweet); err != nil {
		log.Fatal(err)
	}

	text := tweet.Data.Text
	if tweet.Data.NoteTweet.Text != "" {
		text = tweet.Data.NoteTweet.Text
	}

	t := Tweet{
		ID:        tweet.Data.ID,
		CreatedAt: tweet.Data.CreatedAt,
		Username:  tweet.Includes.Users[0].Username,
		Text:      text,
	}

	var quotedTweets []Tweet
	var repliedTweets []Tweet

	for i, r := range tweet.Data.ReferencedTweets {
		if r.Type == "quoted" {
			q := tweet.Includes.Tweets[i]
			text := q.Text
			if q.NoteTweet.Text != "" {
				text = q.NoteTweet.Text
			}

			quotedTweets = append(quotedTweets, Tweet{
				ID:        q.ID,
				CreatedAt: q.CreatedAt,
				AuthorID:  q.AuthorID,
				Text:      text,
			})
		} else if r.Type == "replied_to" {
			r := tweet.Includes.Tweets[i]
			text := r.Text
			if r.NoteTweet.Text != "" {
				text = r.NoteTweet.Text
			}

			repliedTweets = append(repliedTweets, Tweet{
				ID:        r.ID,
				CreatedAt: r.CreatedAt,
				AuthorID:  r.AuthorID,
				Text:      text,
			})
		}
	}

	d := &document.Document{}

	var content bytes.Buffer

	// TODO: probably have different documents per tweet, linked by metadata through IDs
	for _, replied := range repliedTweets {
		content.WriteString(fmt.Sprintf("> %s\n", replied.Text))
		content.WriteString(fmt.Sprintf("> - %s\n", replied.Username))
	}

	content.WriteString(t.Text)

	for _, quoted := range quotedTweets {
		content.WriteString("Quoted tweet:\n")
		content.WriteString(fmt.Sprintf("> %s\n", quoted.Text))
		content.WriteString(fmt.Sprintf("> - %s\n", quoted.Username))
	}

	tweetURL := fmt.Sprintf("https://twitter.com/%s/status/%s", t.Username, t.ID)

	d.Content = content.Bytes()
	d.Metadata.ID = t.ID
	d.Metadata.Title = t.ID
	d.Metadata.Authors = []string{t.Username}
	d.Metadata.PublishedTime = &t.CreatedAt
	d.Metadata.ProcessedTime = time.Now().Format(time.RFC3339)
	d.Metadata.Source = tweetURL
	d.Metadata.Type = document.TypeTweet

	return d, nil
}
