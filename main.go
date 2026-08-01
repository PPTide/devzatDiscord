package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	api "github.com/quackduck/devzat/devzatapi"
	"golang.org/x/image/draw"
)

var config = struct {
	botToken string
	url      string
	port     string
}{}

func init() {
	config.botToken = EnvOrPanic("DISCORD_BOT_TOKEN")
	config.url = EnvOrPanic("AVATAR_URL")
	config.port = EnvOrDefault("AVATAR_PORT", "8080")
}

func EnvOrDefault(key, defaultValue string) string {
	v, ok := os.LookupEnv(key)
	if ok {
		return v
	}
	return defaultValue
}

func EnvOrPanic(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		panic(fmt.Sprintf("%s environment variable not set", key))
	}
	return v
}

var idToContextCancel = make(map[uint64]context.CancelFunc)

var db *sql.DB
var dbLock = sync.Mutex{}

type messageSendFunc func(api.Message)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbLock.Lock()
	var err error
	db, err = sql.Open("sqlite3", "./db.sqlite")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS servers
(
    id         integer not null
            primary key autoincrement,
    guild_id   TEXT    not null
            unique,
    channel_id TEXT    not null
            unique,
    devzat_url TEXT    not null,
    devzat_key TEXT    not null
);`)
	if err != nil {
		panic(err)
	}

	dis := setupDiscordBase(ctx)
	defer dis.close()
	fmt.Println("Discord connected")

	query, err := db.Query("SELECT id, guild_id, channel_id, devzat_url, devzat_key FROM servers")
	if err != nil {
		panic(err)
	}
	dbLock.Unlock()

	for query.Next() {
		var (
			id        uint64
			guildID   string
			channelID string
			devzatURL string
			devzatKey string
		)

		err := query.Scan(&id, &guildID, &channelID, &devzatURL, &devzatKey)
		if err != nil {
			panic(err)
		}

		var sendToDiscord messageSendFunc
		var sendToDevzat messageSendFunc

		ctx, cancel := context.WithCancel(ctx)
		idToContextCancel[id] = cancel

		sendToDiscord, bgFuncDis := dis.setupDiscord(channelID, &sendToDevzat, ctx)
		sendToDevzat, bgFuncDev := setupDevzat(devzatURL, devzatKey, &sendToDiscord, ctx)

		go bgFuncDis()
		go bgFuncDev()
	}

	http.HandleFunc("/avatar/{small}", func(w http.ResponseWriter, r *http.Request) {
		small := r.PathValue("small")

		img, err := png.Decode(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(small)))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Invalid image format"))
			return
		}

		dst := image.NewNRGBA(image.Rect(0, 0, 256, 256))

		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

		w.Header().Set("Content-Type", "image/png")
		err = png.Encode(w, dst)
		if err != nil {
			panic(err)
		}
	})

	err = http.ListenAndServe(":"+config.port, nil)
	if err != nil {
		panic(err)
	}
}

func startNewBridgeForGuild(guildID string, newChannelID string, newDevzatURL string, newDevzatKey string, d *Discord, ctx context.Context) {
	dbLock.Lock()
	defer dbLock.Unlock()

	if newDevzatURL == "" {
		newDevzatURL = "devzat.hackclub.com:5556"
	}

	_, err := db.Exec(`
INSERT OR IGNORE INTO servers (guild_id, channel_id, devzat_url, devzat_key) VALUES (?, ?, ?, ?);
UPDATE servers SET
	channel_id=?,
	devzat_url=?,
	devzat_key=?
WHERE guild_id=?`, guildID, newChannelID, newDevzatURL, newDevzatKey, newChannelID, newDevzatURL, newDevzatKey, guildID)
	if err != nil {
		panic(err)
	}

	row := db.QueryRow(`SELECT id FROM servers WHERE guild_id=?`, guildID)

	var id uint64
	err = row.Scan(&id)
	if err != nil {
		panic(err)
	}

	cancel, ok := idToContextCancel[id]
	if ok {
		cancel()
	}

	var sendToDiscord messageSendFunc
	var sendToDevzat messageSendFunc

	ctx, cancel = context.WithCancel(ctx)
	idToContextCancel[id] = cancel

	sendToDiscord, bgFuncDis := d.setupDiscord(newChannelID, &sendToDevzat, ctx)
	sendToDevzat, bgFuncDev := setupDevzat(newDevzatURL, newDevzatKey, &sendToDiscord, ctx)

	go bgFuncDis()
	go bgFuncDev()
}

func stringWithMaxLength(s string, length int) string {
	if len(s) < length {
		return s
	}
	return s[:length]
}
