package twitter

import (
	"encoding/json"
	"testing"
)

const tweetData = `{
  "data": {
    "created_at": "2025-01-01T12:23:00.000Z",
    "author_id": "125083073",
    "edit_history_tweet_ids": [
      "1874431128096096666"
    ],
    "referenced_tweets": [
      {
        "type": "quoted",
        "id": "1874159800566825026"
      }
    ],
    "id": "1874431128096096666",
    "text": "The only pending behemoth is @sporedotfun. Nothing even close is being done. https://t.co/k3z9y3G9wy"
  },
  "includes": {
    "users": [
      {
        "id": "125083073",
        "name": "Bruno :: ▬▬ι═══════ﺤ⋆｡𖦹°⭒˚｡⋆",
        "username": "bitfalls"
      }
    ],
    "tweets": [
      {
        "note_tweet": {
          "text": "Whether it's Virtuals, Ai16z or an undiscovered agentic behemoth yet to emerge, I whole heartedly agree with this quote:\n\n\"it is totally possible in coming years to see an agentic nation that has a larger GDP than the top countries in our current world\"\n\nFor context, last year:\n\n- USA GDP was $29 trillion\n- VISA total tx volume was $6.8 trillion\n\nThe level of agentic economic activity will be insane; we've only recently saw a glimpse of on-chain agent-to-agent commercial transactions ( @luna_virtuals  , @agent_stix , etc)\n\nThe latest @a16zcrypto State of Crypto report highlighted that stablecoins facilitated around $8.5 trillion in transaction volume in Q2 2024, which was more than double Visa’s $3.9 trillion for the same period\n\nExpect this stablecoin activity to rocket as more AI Agents use crypto rails\n\nAlongside every other adjacent vertical (DePin etc)\n\nThe cat's out of the bag; 2025 is going to be a wild ride",
          "entities": {
            "mentions": [
              {
                "start": 491,
                "end": 505,
                "username": "luna_virtuals",
                "id": "1674751108265148417"
              },
              {
                "start": 509,
                "end": 520,
                "username": "agent_stix",
                "id": "1865947371529445376"
              },
              {
                "start": 540,
                "end": 551,
                "username": "a16zcrypto",
                "id": "1539681011696603137"
              }
            ]
          }
        },
        "created_at": "2024-12-31T18:24:51.000Z",
        "author_id": "223921570",
        "edit_history_tweet_ids": [
          "1874159800566825026"
        ],
        "referenced_tweets": [
          {
            "type": "quoted",
            "id": "1874111116403761316"
          }
        ],
        "id": "1874159800566825026",
        "text": "Whether it's Virtuals, Ai16z or an undiscovered agentic behemoth yet to emerge, I whole heartedly agree with this quote:\n\n\"it is totally possible in coming years to see an agentic nation that has a larger GDP than the top countries in our current world\"\n\nFor context, last year:… https://t.co/UIMJL02j8k https://t.co/tsz2UP1TwB https://t.co/iR4AfY0tBp"
      }
    ]
  }
}`

func TestGetTweet(t *testing.T) {
	tweet, _ := GetTweet("1874431128096096666")

	json, err := json.MarshalIndent(tweet, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(json))
}
