package main

import (
	"context"

	api "github.com/quackduck/devzat/devzatapi"
)

func setupDevzat(send chan<- api.Message, receive <-chan api.Message, ctx context.Context) {
	devSess, err := api.NewSession(config.devzatURL, config.devzatApiKey)
	if err != nil {
		panic(err)
	}

	listener, _, err := devSess.RegisterListener(false, false, "", api.WithSystemMessages(), api.WithColorNames())
	if err != nil {
		panic(err)
	}

	for {
		select {
		case msg := <-listener:
			send <- msg
		case msg := <-receive:
			err := devSess.SendMessage(msg)
			if err != nil {
				panic(err)
			}
		case <-ctx.Done():
			return
		}
	}
}
