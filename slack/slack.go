package slack

import (
	"regexp"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/mempirate/scholar/log"
)

// https://stackoverflow.com/a/3809435
const URL_REGEX = `https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`

const (
	ReplyMissingURL = "There doesn't seem to be a URL in your message."
)

type EventType string

const (
	// EventTypeUpload requires a file to be uploaded.
	EventTypeUpload EventType = "upload"
	// EventTypePrompt is a prompt event, where the bot is addressed.
	EventTypePrompt EventType = "prompt"
	// EventTypeMessage is a message event, without addressing the bot.
	EventTypeMessage EventType = "message"
)

// HandlerEvent is an event originating from a handler.
type HandlerEvent struct {
}

type SlackHandler struct {
	log    zerolog.Logger
	client *socketmode.Client
	events chan HandlerEvent

	urlRegex *regexp.Regexp

	threadCache map[string]struct{}
}

func NewSlackHandler(appToken, botToken string) *SlackHandler {
	api := slack.New(
		botToken,
		// slack.OptionDebug(true),
		// slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(
		api,
		// socketmode.OptionDebug(true),
		// socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	return &SlackHandler{
		log:      log.NewLogger("slack"),
		client:   client,
		urlRegex: regexp.MustCompile(URL_REGEX),
		// TODO: bound map
		threadCache: make(map[string]struct{}),
	}
}

// EventStream returns a channel that receives events from Slack.
func (s *SlackHandler) EventStream() chan HandlerEvent {
	return s.events
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
	// Thread ID is determined by the timestamp
	threadID := event.TimeStamp

	// Extract all URLs
	url := s.urlRegex.FindString(event.Text)

	if url == "" {
		s.log.Debug().Str("text", event.Text).Msg("Ignoring event without URL")
		s.client.PostMessage(event.Channel, slack.MsgOptionText(ReplyMissingURL, false), slack.MsgOptionTS(threadID))
		return nil
	}

	s.log.Info().Str("url", url).Str("ts", threadID).Msg("New URL received, starting upload...")

	// TODO:
	// - Cache parentTs
	// - Emit event and spawn goroutine to handle response

	return nil
}

func (s *SlackHandler) onMessage(event *slackevents.MessageEvent) error {
	s.log.Info().Str("text", event.Text).Msg("Received message event")

	return nil
}
