package slack

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/mempirate/scholar/backend"
	"github.com/mempirate/scholar/log"
	"github.com/mempirate/scholar/util"
)

// https://stackoverflow.com/a/3809435
const URL_REGEX = `https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`

const (
	ReplyMissingURL     = "There doesn't seem to be a URL in your message."
	ReplyInvalidURL     = "The URL you provided is invalid. Please provide a valid URL."
	ReplyDownloadFailed = "Failed to download the PDF. Please try again later."
)

type SlackHandler struct {
	log     zerolog.Logger
	client  *socketmode.Client
	backend backend.ScholarBackend

	urlRegex *regexp.Regexp

	// TODO: limit this map
	processingCache map[string]struct{}
}

func NewSlackHandler(appToken, botToken string, backend backend.ScholarBackend) *SlackHandler {
	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))

	client := socketmode.New(api)

	return &SlackHandler{
		log:             log.NewLogger("slack"),
		client:          client,
		urlRegex:        regexp.MustCompile(URL_REGEX),
		backend:         backend,
		processingCache: make(map[string]struct{}),
	}
}

func (s *SlackHandler) Start() {
	go s.client.Run()

	for evt := range s.client.Events {
		switch evt.Type {
		case socketmode.EventTypeConnecting:
			s.log.Debug().Msg("Connecting to Slack with Socket Mode...")
		case socketmode.EventTypeConnectionError:
			s.log.Warn().Any("data", evt.Data).Msg("Connection failed. Retrying later...")
		case socketmode.EventTypeConnected:
			s.log.Info().Msg("Connected to Slack with Socket Mode")
		case socketmode.EventTypeEventsAPI:
			apiEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				s.log.Warn().Msg("Ignored event")
				continue
			}

			if err := s.onEvent(apiEvent); err != nil {
				s.log.Error().Err(err).Msg("Failed to handle event, will be retried")
				continue
			}

			s.log.Debug().Msg("Event handled successfully, sending ack")
			// Acknowledge the event so it doesn't get retried
			s.client.Ack(*evt.Request)
		default:
			s.log.Trace().Str("type", string(evt.Type)).Msg("Ignored event")
		}
	}
}

// onEvent handles an incoming event from Slack. If this returns an error, the event should not be acknowledged
// to force a retry.
func (s *SlackHandler) onEvent(event slackevents.EventsAPIEvent) error {
	if event.Type == slackevents.CallbackEvent {
		callbackEvent := event.InnerEvent
		switch ev := callbackEvent.Data.(type) {
		case *slackevents.MessageEvent:
			return s.onMessage(ev)
		case *slackevents.AppMentionEvent:
			return s.onAppMention(ev)
		default:
			s.log.Debug().Str("type", callbackEvent.Type).Msg("Unhandled callback event")
		}
	}

	// If we get here, just ignore and acknowledge
	return nil
}

func (s *SlackHandler) onAppMention(event *slackevents.AppMentionEvent) error {
	// TODO: check if this is a message inside a thread, or a thread starter!
	// Thread ID is determined by the timestamp
	threadID := event.TimeStamp

	// Check if we're already processing this thread
	if _, ok := s.processingCache[threadID]; ok {
		return nil
	}

	// Else add to cache
	s.processingCache[threadID] = struct{}{}

	// Extract all URLs
	urlStr := s.urlRegex.FindString(event.Text)

	if urlStr == "" {
		s.log.Debug().Str("text", event.Text).Msg("Ignoring event without URL")
		s.client.PostMessage(event.Channel, slack.MsgOptionText(ReplyMissingURL, false), slack.MsgOptionTS(threadID))
		return nil
	}

	url, err := url.Parse(urlStr)
	if err != nil {
		s.log.Error().Err(err).Str("url", urlStr).Msg("Failed to parse URL")
		s.client.PostMessage(event.Channel, slack.MsgOptionText(ReplyInvalidURL, false), slack.MsgOptionTS(threadID))
		return nil
	}

	s.log.Info().Str("url", urlStr).Str("ts", threadID).Msg("New URL received, starting upload...")

	path, err := util.DownloadPDF(url)
	if err != nil {
		s.log.Error().Err(err).Str("url", urlStr).Msg("Failed to download PDF")
		// TODO: See if error is retryable or user-facing, and respond accordingly
		s.client.PostMessage(event.Channel, slack.MsgOptionText(fmt.Sprintf("%s (error: %s)", ReplyDownloadFailed, err), false), slack.MsgOptionTS(threadID))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s.backend.CreateThread(ctx, threadID)

	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := s.backend.UploadFile(ctx, threadID, path); err != nil {
		s.log.Error().Err(err).Msg("Failed to upload file")
		// This should def be retried
		return err
	}

	s.log.Debug().Str("url", urlStr).Msg("PDF uploaded successfully, prompting for summary...")

	summary, err := s.backend.Prompt(ctx, threadID, fmt.Sprintf("Please provide a summary of this file: %s. Disregard the path. Always mention the title of the paper, not the file name. Only use a single reference per unique file.", path))
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to prompt for summary")
		return err
	}

	// TODO: error handling (also wrap this in func)
	s.client.PostMessage(event.Channel, slack.MsgOptionText(summary, true), slack.MsgOptionTS(threadID), slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
		Markdown: true,
	}))

	return nil
}

func (s *SlackHandler) onMessage(event *slackevents.MessageEvent) error {
	s.log.Info().Str("text", event.Text).Msg("Received message event")

	return nil
}
