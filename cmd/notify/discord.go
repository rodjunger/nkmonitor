package notify

import (
	"errors"
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/webhook"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rodjunger/nkmonitor"
)

const (
	checkMark = "✓"
)

type DiscordNotifyer struct {
	w webhook.Client
}

func NewDiscordNotifyer(webhookUrl string) (*DiscordNotifyer, error) {
	client, err := webhookClientFromUrl(webhookUrl)
	if err != nil {
		return nil, err
	}
	return &DiscordNotifyer{client}, nil
}

func webhookClientFromUrl(webhookUrl string) (webhook.Client, error) {
	if webhookUrl == "" {
		return nil, errors.New("empty webhook")
	}

	urlParts := strings.Split(webhookUrl, "/")

	//This feels very wrong
	if len(urlParts) != 7 {
		return nil, errors.New("invalid webhook")
	}

	snowFlake, err := snowflake.Parse(urlParts[5])

	if err != nil {
		return nil, err
	}

	token := urlParts[6]

	client := webhook.New(snowFlake, token)

	return client, nil
}

func sizeToString(size *nkmonitor.SizeInfo) string {
	if size == nil {
		return ""
	}

	symbol := "x"
	if size.Restocked {
		symbol = checkMark
	}

	return fmt.Sprintf("%s %s %s", size.Description, size.Sku, symbol)
}

func (d *DiscordNotifyer) Notify(info nkmonitor.RestockInfo) error {
	if d == nil {
		return errors.New("nil instance")
	}

	var availableSizes, inStockSizes []string

	for i := range info.Sizes { //Using i to avoid problems with reused loop variable and pointers
		infoStr := sizeToString(info.Sizes[i])
		if info.Sizes[i].IsAvailable {
			availableSizes = append(availableSizes, infoStr)
		} else {
			inStockSizes = append(inStockSizes, infoStr)
		}
	}

	webHook := discord.NewEmbedBuilder().SetTitle(info.Name+" just restocked!").
		SetColor(65280).
		SetFooterText("Powered by the openMonitors project").
		SetThumbnail(info.Picture).
		SetURL("https://www.nike.com.br"+info.Path).
		AddField("Price", info.Price, true).
		AddField("Code", info.Code, true)
	if len(availableSizes) > 0 {
		webHook.AddField("Available sizes (can be added to cart)", strings.Join(availableSizes, "\n"), false)
	}
	if len(inStockSizes) > 0 {
		webHook.AddField("In stock sizes (can't be added to cart)", strings.Join(inStockSizes, "\n"), false)
	}

	if _, err := d.w.CreateEmbeds([]discord.Embed{webHook.Build()}); err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}
