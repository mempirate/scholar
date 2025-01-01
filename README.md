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
