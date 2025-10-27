package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type AIRequest struct {
	Displayname string
	Message     string
	RoomID      id.RoomID
	Sender      string
	Timestamp   time.Time
}

type AIResponse struct {
	Message string
	RoomID  id.RoomID
}

type MatrixClient struct {
	aiRequestChannel  chan AIRequest
	aiResponseChannel chan AIResponse
	client            *mautrix.Client
	started           time.Time
	user              string
}

func NewMatrixClient(log Logger, homeserver string, user string, accessToken string, aiRequestChannel chan AIRequest, aiResponseChannel chan AIResponse) (*MatrixClient, error) {
	userID, err := parseUserID(user)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse user ID '%s': %v", user, err)
	}

	matrixClient, err := mautrix.NewClient(homeserver, userID, accessToken)
	if err != nil {
		panic(err)
	}
	matrixClient.Log = log
	matrixClient.StateStore = mautrix.NewMemoryStateStore()
	return &MatrixClient{
		aiRequestChannel:  aiRequestChannel,
		aiResponseChannel: aiResponseChannel,
		client:            matrixClient,
		started:           time.Now(),
		user:              user,
	}, nil
}

func (m *MatrixClient) Run(ctx context.Context, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	go func() {
		waitGroup.Add(1)
		defer waitGroup.Done()
		for {
			select {
			case <-ctx.Done():
				m.client.Log.Info().Msg("Exiting response handler.")
				return
			case resp := <-m.aiResponseChannel:
				m.handleResponse(ctx, resp)
			}
		}
	}()
	syncer := m.client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, m.onMessage)
	err := m.client.SyncWithContext(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		m.client.Log.Error().Err(err).Msg("Failed to run sync with context.")
	}
}

func (m *MatrixClient) onMessage(ctx context.Context, evt *event.Event) {
	eventTime := timestampToTime(evt.Timestamp)
	if m.started.After(eventTime) {
		return
	}
	if evt.Sender.String() == m.user {
		return
	}
	profile, err := m.client.GetProfile(ctx, evt.Sender)
	displayname := "John Doe"
	if err != nil {
		displayname = evt.Sender.String()
	} else if profile.DisplayName != "" {
		displayname = profile.DisplayName
	} else {
		displayname = "Harry Styles"
	}
	since := eventTime.Sub(m.started)
	m.client.Log.Info().
		Str("sender", evt.Sender.String()).
		Int64("timestamp", evt.Timestamp).
		Str("ago", since.String()).
		Str("name", displayname).
		Str("type", evt.Type.String()).
		Str("id", evt.ID.String()).
		Str("body", evt.Content.AsMessage().Body).
		Msg("Received message")
	_, err = m.client.UserTyping(context.TODO(), evt.RoomID, true, 10*time.Second)
	if err != nil {
		m.client.Log.Error().Err(err).Msg("Failed to set typing")
	}
	m.aiRequestChannel <- AIRequest{
		Displayname: displayname,
		Message:     evt.Content.AsMessage().Body,
		RoomID:      evt.RoomID,
		Sender:      evt.Sender.String(),
		Timestamp:   eventTime,
	}
}

func (m *MatrixClient) handleResponse(ctx context.Context, resp AIResponse) {
	var roomID id.RoomID = resp.RoomID
	e, err := m.client.SendText(ctx, roomID, resp.Message)
	if err != nil {
		m.client.Log.Error().Err(err).Msg("Failed to send event")
	} else {
		m.client.Log.Info().Stringer("event_id", e.EventID).Msg("Event sent")
	}
	_, err = m.client.UserTyping(ctx, resp.RoomID, false, 1*time.Second)
	if err != nil {
		m.client.Log.Error().Err(err).Msg("Failed to unset typing")
	}
}

func parseUserID(username string) (id.UserID, error) {
	var result id.UserID
	// Check if the string starts with "@"
	if !strings.HasPrefix(username, "@") {
		return result, fmt.Errorf("invalid username format: must start with @")
	}

	// Find the position of ":"
	colonIndex := strings.Index(username, ":")
	if colonIndex == -1 {
		return result, fmt.Errorf("invalid username format: missing domain separator :")
	}

	// Extract the local part (removing the @ prefix)
	localPart := username[1:colonIndex]

	// Extract the domain part
	domainPart := username[colonIndex+1:]

	// Validate that both parts are non-empty
	if localPart == "" {
		return result, fmt.Errorf("invalid username format: empty local part")
	}
	if domainPart == "" {
		return result, fmt.Errorf("invalid username format: empty domain part")
	}

	return id.NewUserID(localPart, domainPart), nil
}

// / Convert a Matrix timestamp, which is milliseconds since Unix epoch
// / to a golang Time
func timestampToTime(milliseconds int64) time.Time {
	// Convert milliseconds to seconds and nanoseconds
	seconds := milliseconds / 1000
	nanoseconds := (milliseconds % 1000) * int64(time.Millisecond) // Convert millisecond remainder to nanoseconds

	// Create time.Time object using Unix time
	return time.Unix(seconds, nanoseconds)
}
