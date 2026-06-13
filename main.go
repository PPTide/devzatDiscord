package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"os"

	discord "github.com/bwmarrin/discordgo"
	"github.com/leaanthony/go-ansi-parser"
	api "github.com/quackduck/devzat/devzatapi"
	"golang.org/x/image/draw"
)

var config = struct {
	botToken     string
	appId        string
	channelId    string
	url          string
	port         string
	devzatURL    string
	devzatApiKey string
}{}

func init() {
	config.botToken = EnvOrPanic("DISCORD_BOT_TOKEN")
	config.appId = EnvOrPanic("DISCORD_APP_ID")
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
	devSess, err := api.NewSession(config.devzatURL, config.devzatApiKey)
	if err != nil {
		panic(err)
	}

	listener, _, err := devSess.RegisterListener(false, false, "", api.WithSystemMessages(), api.WithColorNames())
	if err != nil {
		panic(err)
	}

	disSess, err := discord.New("Bot " + config.botToken)
	if err != nil {
		panic(err)
	}

	webhook, err := disSess.ChannelWebhooks(config.channelId)
	//webhook, err := disSess.WebhookCreate(config.channelId, "Devzat Webhook", "")
	if err != nil {
		panic(err)
	}

	var thisAppWebhook *discord.Webhook = nil
	for _, w := range webhook {
		if w.ApplicationID == config.appId {
			thisAppWebhook = w
		}
	}

	if thisAppWebhook == nil {
		thisAppWebhook, err = disSess.WebhookCreate(config.channelId, "Devzat Bridge", "")
		if err != nil {
			panic(err)
		}
	}

	disSess.AddHandler(func(s *discord.Session, msg *discord.MessageCreate) {
		if msg.ChannelID != config.channelId {
			return
		}
		if msg.WebhookID == thisAppWebhook.ID {
			return
		}

		err := devSess.SendMessage(api.Message{
			Room: "#main",
			From: "\x1b[95mD@\x1b[0m " + msg.Author.Username,
			Data: msg.Content,
		})
		if err != nil {
			fmt.Printf("Error sending message: %s\n", err)
		}
	})
	disSess.Identify.Intents = discord.IntentsGuildMessages

	err = disSess.Open()
	if err != nil {
		panic(err)
	}
	defer func(disSess *discord.Session) {
		err := disSess.Close()
		if err != nil {
			panic(err)
		}
	}(disSess)

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

	go func() {
		err := http.ListenAndServe(":"+config.port, nil)
		if err != nil {
			panic(err)
		}
	}()

	for {
		select {
		case msg := <-listener:
			cleanSender, err := ansi.Cleanse(msg.From)
			if err != nil {
				panic(err)
			}
			cleanMsg, err := ansi.Cleanse(msg.Data)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Listener received: %+v\n", msg)
			_, err = disSess.WebhookExecute(thisAppWebhook.ID, thisAppWebhook.Token, true, &discord.WebhookParams{
				Content:         cleanMsg,
				Username:        stringWithMaxLength(cleanSender, 80),
				AvatarURL:       getAvatarLink(msg.From),
				TTS:             false,
				Files:           nil,
				Components:      nil,
				Embeds:          nil,
				Attachments:     nil,
				AllowedMentions: nil,
				Flags:           0,
				ThreadName:      "",
			})
			if err != nil {
				panic(fmt.Sprintf("error executing webhook %+v", err))
			}
		}
	}
}

func getAvatarImage(name string) image.Image {
	name = stringWithMaxLength(name, 256)
	if name == "" {
		img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, image.Transparent)
		return img
	}
	styledTexts, err := ansi.Parse(name)
	if err != nil {
		panic(err)
	}

	img := image.NewNRGBA(image.Rect(0, 0, len(styledTexts), 3))

	for x, styledText := range styledTexts {
		if styledText.FgCol == nil {
			styledText.FgCol = ansi.Cols[7]
		}

		img.Set(x, 1, color.RGBA{R: styledText.FgCol.Rgb.R, G: styledText.FgCol.Rgb.G, B: styledText.FgCol.Rgb.B, A: 255})
		if styledText.BgCol != nil {
			img.Set(x, 0, color.RGBA{R: styledText.BgCol.Rgb.R, G: styledText.BgCol.Rgb.G, B: styledText.BgCol.Rgb.B, A: 255})
			img.Set(x, 2, color.RGBA{R: styledText.BgCol.Rgb.R, G: styledText.BgCol.Rgb.G, B: styledText.BgCol.Rgb.B, A: 255})
		} else {
			img.Set(x, 0, color.RGBA{R: styledText.FgCol.Rgb.R, G: styledText.FgCol.Rgb.G, B: styledText.FgCol.Rgb.B, A: 255})
			img.Set(x, 2, color.RGBA{R: styledText.FgCol.Rgb.R, G: styledText.FgCol.Rgb.G, B: styledText.FgCol.Rgb.B, A: 255})
		}
	}

	return img
}

func getAvatarLink(name string) string {
	img := getAvatarImage(name)

	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		panic(err)
	}

	return config.url + "avatar/" + url.PathEscape(base64.StdEncoding.EncodeToString(buf.Bytes()))
}

func stringWithMaxLength(s string, length int) string {
	if len(s) < length {
		return s
	}
	return s[:length]
}
