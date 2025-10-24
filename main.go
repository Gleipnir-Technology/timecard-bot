package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	//"github.com/openai/openai-go/v3/shared"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exzerolog"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

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

func main() {
	accessToken := os.Getenv("MATRIX_ACCESS_TOKEN")
	databaseURI := os.Getenv("POSTGRES_URI")
	homeserver := os.Getenv("MATRIX_HOMESERVER")
	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	user := os.Getenv("MATRIX_USER")

	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		//w.Out = io.Writer(buf)
		w.TimeFormat = time.Stamp
	})).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	exzerolog.SetupDefaults(&log)

	if accessToken == "" {
		log.Error().Msg("You must provide a value for MATRIX_ACCESS_TOKEN")
		os.Exit(1)
	}
	if databaseURI == "" {
		log.Error().Msg("You must provide a value for POSTGRES_URI")
		os.Exit(1)
	}
	if homeserver == "" {
		log.Error().Msg("You must provide a value for MATRIX_HOMESERVER")
		os.Exit(1)
	}
	if openaiApiKey == "" {
		log.Error().Msg("You must provide a value for OPENAI_API_KEY")
		os.Exit(1)
	}
	if user == "" {
		log.Error().Msg("You must provide a value for MATRIX_USER")
		os.Exit(1)
	}

	userID, err := parseUserID(user)
	if err != nil {
		log.Error().Str("error", "Failed to parse user ID").
			Str("value", user)
		os.Exit(1)
	}

	needsMigrations, err := needsMigrations(databaseURI)
	if err != nil || needsMigrations == nil {
		log.Error().Msg("Failed to check if we need database migrations")
		os.Exit(1)
	}
	if *needsMigrations {
		doMigrations(databaseURI)
		log.Info().Msg("Handled migrations")
	} else {
		log.Info().Msg("No migrations necessary")
	}

	openAIClient := openai.NewClient(
		option.WithAPIKey(openaiApiKey),
	)

	matrixClient, err := mautrix.NewClient(homeserver, userID, accessToken)
	if err != nil {
		panic(err)
	}
	matrixClient.Log = log

	started := time.Now()

	var lastRoomID id.RoomID
	syncer := matrixClient.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		eventTime := timestampToTime(evt.Timestamp)
		if started.After(eventTime) {
			return
		}
		if evt.Sender.String() == user {
			return
		}
		lastRoomID = evt.RoomID
		since := eventTime.Sub(started)
		log.Info().
			Str("sender", evt.Sender.String()).
			Int64("timestamp", evt.Timestamp).
			Str("ago", since.String()).
			Str("type", evt.Type.String()).
			Str("id", evt.ID.String()).
			Str("body", evt.Content.AsMessage().Body).
			Msg("Received message")
		_, err := matrixClient.UserTyping(context.TODO(), lastRoomID, true, 10*time.Second)
		if err != nil {
			log.Error().Err(err).Msg("Failed to set typing")
		}
		chatCompletion, err := openAIClient.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Say this is a test"),
			},
			Model: openai.ChatModelGPT4o,
		})
		if err != nil {
			panic(err.Error())
		}
		resp, err := matrixClient.SendText(context.TODO(), lastRoomID, chatCompletion.Choices[0].Message.Content)
		if err != nil {
			log.Error().Err(err).Msg("Failed to send event")
		} else {
			log.Info().Stringer("event_id", resp.EventID).Msg("Event sent")
		}
		_, err = matrixClient.UserTyping(context.TODO(), lastRoomID, false, 1*time.Second)
		if err != nil {
			log.Error().Err(err).Msg("Failed to unset typing")
		}
	})

	resp, err := matrixClient.GetOwnDisplayName(context.TODO())
	if err != nil {
		panic(err)
	}
	log.Printf("Display name: %s", resp.DisplayName)
	syncCtx, cancelSync := context.WithCancel(context.Background())
	var syncStopWait sync.WaitGroup
	syncStopWait.Add(1)

	go func() {
		err = matrixClient.SyncWithContext(syncCtx)
		defer syncStopWait.Done()
		if err != nil && !errors.Is(err, context.Canceled) {
			panic(err)
		}
	}()
	for {
		//line, err := rl.Readline()
		if err != nil { // io.EOF
			break
		}
		if lastRoomID == "" {
			//log.Error().Msg("Wait for an incoming message before sending messages")
			time.Sleep(1 * time.Second)
			continue
		}
		/*resp, err := matrixClient.SendText(context.TODO(), lastRoomID, "hey")
		if err != nil {
			log.Error().Err(err).Msg("Failed to send event")
		} else {
			log.Info().Stringer("event_id", resp.EventID).Msg("Event sent")
		}*/
	}
	cancelSync()
	syncStopWait.Wait()

	log.Info().Msg("Success")
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
