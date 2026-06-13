package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/url"

	discord "github.com/bwmarrin/discordgo"
	"github.com/leaanthony/go-ansi-parser"
	api "github.com/quackduck/devzat/devzatapi"
)

type Discord struct {
	session *discord.Session

	listeners map[string]chan *discord.MessageCreate
}

func (d *Discord) registerListener(channelId string) <-chan *discord.MessageCreate {
	listener := make(chan *discord.MessageCreate)
	d.listeners[channelId] = listener
	return listener
}

func (d *Discord) sendFromDiscord(channelId string, msg *discord.MessageCreate) {
	d.listeners[channelId] <- msg
}

func (d *Discord) close() {
	err := d.session.Close()
	if err != nil {
		panic(err)
	}

}

func setupDiscordBase() *Discord {
	dis := Discord{
		listeners: make(map[string]chan *discord.MessageCreate),
	}

	var err error
	dis.session, err = discord.New("Bot " + config.botToken)
	if err != nil {
		panic(err)
	}

	dis.session.AddHandler(func(s *discord.Session, msg *discord.MessageCreate) {
		if msg.ChannelID != config.channelId {
			return
		}
		dis.sendFromDiscord(msg.ChannelID, msg)
	})
	dis.session.Identify.Intents = discord.IntentsGuildMessages

	err = dis.session.Open()
	if err != nil {
		panic(err)
	}

	return &dis
}

func (d *Discord) setupDiscord(send chan<- api.Message, receive <-chan api.Message, ctx context.Context) {
	webhook, err := d.session.ChannelWebhooks(config.channelId)
	//webhook, err := disSess.WebhookCreate(config.channelId, "Devzat Webhook", "")
	if err != nil {
		panic(err)
	}

	application, err := d.session.Application("@me")
	if err != nil {
		panic(err)
	}

	var thisAppWebhook *discord.Webhook = nil
	for _, w := range webhook {
		if w.ApplicationID == application.ID {
			thisAppWebhook = w
		}
	}

	if thisAppWebhook == nil {
		thisAppWebhook, err = d.session.WebhookCreate(config.channelId, "Devzat Bridge", "")
		if err != nil {
			panic(err)
		}
	}

	discordListener := d.registerListener(config.channelId)

	for {
		select {
		case msg := <-receive:
			cleanSender, err := ansi.Cleanse(msg.From)
			if err != nil {
				panic(err)
			}
			cleanMsg, err := ansi.Cleanse(msg.Data)
			if err != nil {
				panic(err)
			}
			//fmt.Printf("Listener received: %+v\n", msg)
			_, err = d.session.WebhookExecute(thisAppWebhook.ID, thisAppWebhook.Token, true, &discord.WebhookParams{
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
		case msg := <-discordListener:
			if msg.WebhookID == thisAppWebhook.ID {
				continue
			}

			send <- api.Message{
				Room: "#main",
				From: "\x1b[95mD@\x1b[0m " + msg.Author.Username,
				Data: msg.Content,
			}
		case <-ctx.Done():
			return
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
