# Scholar

LLM-powered research assistant capable of summarizing documents, answering questions, and building a knowledge base.
Implemented as a Slack App, with various supported content types.

## What

Scholar is an OpenAI assistant with an associated vector store. Documents can be added to the vector store, which Scholar
can then use to answer questions or summarize documents. It can access the vector store using the [`file_search` tool](https://platform.openai.com/docs/assistants/tools/file-search),
creating a second brain that can be used to store and retrieve information.

When referencing any file in the vector store, the name of that file will be returned in any output from Scholar.

## Slack Integration
The Slack integration currently works with 2 commands:
- `/upload <link>`: Upload content at the provided link to the vector store. Useful if you just want to expand the content available to Scholar.
- `/summary <link>`: Summarize the content at the provided link. This will also upload the content to the vector store.

#### Examples

- Upload a tweet: `/upload https://x.com/Euler__Lagrange/status/1874548493399769376`
- Summarize the bitcoin whitepaper: `/summary https://bitcoin.org/bitcoin.pdf`

## Content Types
Scholar supports the following content types:
- PDFs
- Tweets
- Web pages
  - Regular articles
  - Github markdown files (READMEs, etc.)

With plans to support more in the future (like Github repositories).

### PDFs
PDFs are currently downloaded from the link, verified to be a PDF, and then uploaded to the vector store as is. This
can potentially be improved by extracting the text from the PDF with something like [markitdown](https://github.com/microsoft/markitdown)
and upload that instead. Note sure of the added benefit of this yet.

### Tweets
> [!NOTE]
> With a free plan, you can only call the Twitter API to retrieve a tweet once every 15 minutes.
> If rate-limited, Scholar will return an error message. Will implement caching and load-spreading soon.

Tweets are downloaded using the Twitter v2 API. They can contain references to other tweets up to 1 level deep, for example
if the tweet quotes another tweet, or if the tweet is a reply to another tweet. These references are also downloaded and converted into
the following JSON and uploaded to Scholar:

```json
{
  "id": "1874431128096096666",
  "created_at": "2025-01-01T12:23:00.000Z",
  "username": "elonmusk",
  "author_id": "",
  "text": "The only pending behemoth is @sporedotfun. Nothing even close is being done.",
  "quoted_tweets": [
    {
      "id": "1874159800566825026",
      "created_at": "2024-12-31T18:24:51.000Z",
      "username": "",
      "author_id": "223921570",
      "text": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. ",
    }
  ],
  "replied_tweets": null
}
```

### Web Pages
Web pages are downloaded and converted to Markdown with [`html-to-markdown`](https://github.com/JohannesKaufmann/html-to-markdown).

## Features

#### Content
- [x] Upload & summarize PDFs
- [x] Upload & summarize tweets
- [x] Upload & summarize articles (web pages)
- [ ] Github repositories

#### Slack Integration
- [x] Scholar commands
- [x] Interactivity with mentions
- [ ] Saving messages to the vector store
- [ ] Per-thread context
- [ ] User context
