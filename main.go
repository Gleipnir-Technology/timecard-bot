package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
	homeserver := os.Getenv("MATRIX_HOMESERVER")
	user := os.Getenv("MATRIX_USER")

	userID, err := parseUserID(user)
	if err != nil {
		panic(err)
	}

	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		//w.Out = io.Writer(buf)
		w.TimeFormat = time.Stamp
	})).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	exzerolog.SetupDefaults(&log)

	client, err := mautrix.NewClient(homeserver, userID, accessToken)
	if err != nil {
		panic(err)
	}
	client.Log = log

	var lastRoomID id.RoomID
	syncer := client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		lastRoomID = evt.RoomID
		log.Info().
			Str("sender", evt.Sender.String()).
			Int64("timestamp", evt.Timestamp).
			Str("type", evt.Type.String()).
			Str("id", evt.ID.String()).
			Str("body", evt.Content.AsMessage().Body).
			Msg("Received message")
	})

	resp, err := client.GetOwnDisplayName(context.TODO())
	if err != nil {
		panic(err)
	}
	log.Printf("Display name: %s", resp.DisplayName)
	syncCtx, cancelSync := context.WithCancel(context.Background())
	var syncStopWait sync.WaitGroup
	syncStopWait.Add(1)

	go func() {
		err = client.SyncWithContext(syncCtx)
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
		/*resp, err := client.SendText(context.TODO(), lastRoomID, "hey")
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
