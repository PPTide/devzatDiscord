package main

import (
	"context"

	api "github.com/quackduck/devzat/devzatapi"
)

func setupDevzat(devzatURL, devzatApiKey string, send *messageSendFunc, ctx context.Context) (messageSendFunc, func()) {
	devSess, err := api.NewSession(devzatURL, devzatApiKey)
	if err != nil {
		panic(err)
	}

	listener, _, err := devSess.RegisterListener(false, false, "", api.WithSystemMessages(), api.WithColorNames())
	if err != nil {
		panic(err)
	}

	return func(msg api.Message) {
			err := devSess.SendMessage(msg)
			if err != nil {
				panic(err)
			}
		}, func() {
			for {
				select {
				case msg := <-listener:
					(*send)(msg)
				case <-ctx.Done():
					return
				}
			}
		}
}
