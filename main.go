package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exzerolog"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

var homeserver = flag.String("homeserver", "", "Matrix homeserver")
var accessToken = flag.String("accesstoken", "", "The access token to use")
var username = flag.String("username", "", "Matrix username localpart")
var database = flag.String("database", "mautrix-example.db", "SQLite database path")
var debug = flag.Bool("debug", false, "Enable debug logs")

func main() {
	flag.Parse()

	if *username == "" || *accessToken == "" || *homeserver == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	userID := id.NewUserID(*username, *homeserver)
	client, err := mautrix.NewClient(*homeserver, userID, *accessToken)
	if err != nil {
		panic(err)
	}
	log := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		//w.Out = io.Writer(buf)
		w.TimeFormat = time.Stamp
	})).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if !*debug {
		log = log.Level(zerolog.InfoLevel)
	}
	exzerolog.SetupDefaults(&log)
	client.Log = log

	// You can also store the user/device IDs and access token and put them in the client beforehand instead of using LoginAs.
	//client.UserID = userID
	//client.DeviceID = "..."
	client.AccessToken = *accessToken
	//
	resp, err := client.GetOwnDisplayName(context.TODO())
	if err != nil {
		panic(err)
	}
	log.Printf("Display name: %s", resp.DisplayName)
	log.Info().Msg("Success")
}
