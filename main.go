package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"

	api "github.com/quackduck/devzat/devzatapi"
	"golang.org/x/image/draw"
)

var config = struct {
	botToken     string
	channelId    string
	url          string
	port         string
	devzatURL    string
	devzatApiKey string
}{}

func init() {
	config.botToken = EnvOrPanic("DISCORD_BOT_TOKEN")
	config.channelId = EnvOrPanic("DISCORD_CHANNEL_ID")
	config.url = EnvOrPanic("AVATAR_URL")
	config.port = EnvOrDefault("AVATAR_PORT", "8080")
	config.devzatURL = EnvOrDefault("DEVZAT_URL", "devzat.hackclub.com:5556")
	config.devzatApiKey = EnvOrPanic("DEVZAT_API_KEY")
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

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dis := setupDiscordBase()
	defer dis.close()
	fmt.Println("Discord connected")

	sendToDiscord := make(chan api.Message, 5)
	sendToDevzat := make(chan api.Message, 5)

	go dis.setupDiscord(sendToDevzat, sendToDiscord, ctx)
	go setupDevzat(sendToDiscord, sendToDevzat, ctx)

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

	err := http.ListenAndServe(":"+config.port, nil)
	if err != nil {
		panic(err)
	}
}

func stringWithMaxLength(s string, length int) string {
	if len(s) < length {
		return s
	}
	return s[:length]
}
