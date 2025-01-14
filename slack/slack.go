package slack

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/rs/zerolog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/mempirate/scholar/log"
)

// https://stackoverflow.com/a/3809435 + Claude
const URL_REGEX = `https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`

const (
	ReplyMissingURL     = "There doesn't seem to be a URL in your message."
	ReplyInvalidURL     = "The URL you provided is invalid. Please provide a valid URL."
	ReplyDownloadFailed = "Failed to download the PDF. Please try again later."
)

type SlashCommand = string

const (
	UploadCommand    SlashCommand = "/upload"
	SummarizeCommand SlashCommand = "/summary"
)

// Command represents a processed command from Slack.
type Command struct {
	CommandType SlashCommand
	UserID      string
	ChannelID   string
	URL         *url.URL
}

type EventType = string

const (
	MessageEvent EventType = "message"
	MentionEvent EventType = "mention"
)

// Event represents a processed event from Slack.
type Event struct {
	Type      EventType
	UserID    string
	ChannelID string
	ThreadID  string
	Text      string
}

type SlackHandler struct {
	log    zerolog.Logger
	client *socketmode.Client

	urlRegex *regexp.Regexp

	commandCh chan Command
	eventCh   chan Event

	// TODO: limit this map
	processingCache map[string]struct{}
}

func NewSlackHandler(appToken, botToken string) *SlackHandler {
	api := slack.New(botToken, slack.OptionAppLevelToken(appToken))

	client := socketmode.New(api)

	return &SlackHandler{
		log:             log.NewLogger("slack"),
		client:          client,
		urlRegex:        regexp.MustCompile(URL_REGEX),
		processingCache: make(map[string]struct{}),

		commandCh: make(chan Command, 32),
		eventCh:   make(chan Event, 32),
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

		case socketmode.EventTypeSlashCommand:
			cmd, ok := evt.Data.(slack.SlashCommand)
			if !ok {
				s.log.Warn().Msg("Ignored event")
				continue
			}

			if err := s.onCommand(cmd); err != nil {
				s.log.Error().Err(err).Msg("Failed to handle command, will be retried")
				continue
			}

			s.client.Ack(*evt.Request)

		default:
			s.log.Trace().Str("type", string(evt.Type)).Msg("Ignored event")
		}
	}
}

// SubscribeCommands returns a channel that yields incoming commands.
func (s *SlackHandler) SubscribeCommands() chan Command {
	return s.commandCh
}

// SubscribeEvents returns a channel that yields incoming events.
func (s *SlackHandler) SubscribeEvents() chan Event {
	return s.eventCh
}

// StartUploadThread starts a new thread in the given channel with the given text, and returns the thread ID.
func (s *SlackHandler) StartUploadThread(channelID, userID, text string) (string, error) {
	response := fmt.Sprintf("%s (uploaded by <@%s>)", text, userID)
	_, threadID, err := s.client.PostMessage(channelID, slack.MsgOptionText(response, false))
	if err != nil {
		s.log.Err(err).Msg("Failed to post message")
		return "", err
	}

	return threadID, nil
}

func (s *SlackHandler) PostEphemeral(channelID, userID, text string) error {
	_, err := s.client.PostEphemeral(channelID, userID, slack.MsgOptionText(text, false))
	return err
}

func (s *SlackHandler) ExtractURL(text string) (*url.URL, error) {
	urlStr := s.urlRegex.FindString(text)
	if urlStr == "" {
		return nil, fmt.Errorf("no URL found in text: %s", text)
	}

	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	return url, nil
}

func (s *SlackHandler) PostMessage(channelID string, threadID *string, text string) error {
	var err error
	if threadID == nil {
		_, _, err = s.client.PostMessage(channelID, slack.MsgOptionText(text, false))
	} else {
		_, _, err = s.client.PostMessage(channelID, slack.MsgOptionText(text, false), slack.MsgOptionTS(*threadID))
	}

	return err
}

func (s *SlackHandler) onCommand(cmd slack.SlashCommand) error {
	switch cmd.Command {
	case UploadCommand:
		uri, err := s.ExtractURL(cmd.Text)
		if err != nil {
			return err
		}

		command := Command{
			CommandType: UploadCommand,
			UserID:      cmd.UserID,
			ChannelID:   cmd.ChannelID,
			URL:         uri,
		}

		s.commandCh <- command

	case SummarizeCommand:
		uri, err := s.ExtractURL(cmd.Text)
		if err != nil {
			return err
		}

		command := Command{
			CommandType: SummarizeCommand,
			UserID:      cmd.UserID,
			ChannelID:   cmd.ChannelID,
			URL:         uri,
		}

		s.commandCh <- command

	default:
		s.log.Debug().Str("command", cmd.Command).Msg("Ignoring unknown command")
	}

	return nil
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

func (s *SlackHandler) onMessage(event *slackevents.MessageEvent) error {
	s.log.Info().Str("thread_id", event.ThreadTimeStamp).Str("user", event.User).Msg("Received message event")

	var threadID string
	if event.ThreadTimeStamp != "" {
		threadID = event.ThreadTimeStamp
	}

	s.eventCh <- Event{
		Type:      MessageEvent,
		UserID:    event.User,
		ChannelID: event.Channel,
		ThreadID:  threadID,
		Text:      event.Text,
	}

	return nil
}

func (s *SlackHandler) onAppMention(event *slackevents.AppMentionEvent) error {
	s.log.Info().Str("thread_id", event.ThreadTimeStamp).Str("user", event.User).Msg("Received mention event")

	threadID := event.TimeStamp
	if event.ThreadTimeStamp != "" {
		threadID = event.ThreadTimeStamp
	}

	s.eventCh <- Event{
		Type:      MentionEvent,
		UserID:    event.User,
		ChannelID: event.Channel,
		ThreadID:  threadID,
		Text:      event.Text,
	}

	return nil
}
