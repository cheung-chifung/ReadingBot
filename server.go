package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	postReadyBtn = tb.InlineButton{
		Unique: "ready",
	}
	postDraftBtn = tb.InlineButton{
		Unique: "draft",
	}
	postEditTitleBtn = tb.InlineButton{
		Unique: "edit_title",
	}
	postEditCommentBtn = tb.InlineButton{
		Unique: "edit_comment",
	}
)

type BotServer struct {
	botToken   string
	botChannel string
	botDBFile  string
	bot        *tb.Bot
	postMgr    PostManager
}

type PostCallbackData struct {
	PostID uint `json:"id"`
}

const postListChunkSize = 5
const postPostMaxCount = 2

func NewBotServer(botToken, botChannel, botDBFile string) *BotServer {
	db, err := gorm.Open("sqlite3", botDBFile)
	if err != nil {
		logrus.WithError(err).WithField("bot_db_file", botDBFile).Fatal("cannot open db")
	}
	db.AutoMigrate(&Post{})
	return &BotServer{
		botToken:   botToken,
		botChannel: botChannel,
		botDBFile:  botDBFile,
		postMgr:    NewPostManager(db),
	}
}

func (b *BotServer) Start() error {
	if b.bot != nil {
		return errors.New("server is started")
	}

	var err error
	b.bot, err = tb.NewBot(tb.Settings{
		Token:  b.botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return err
	}

	publishJob := func() {
		_ = b.PublishPosts(postPostMaxCount)
	}

	scheduler.Every().Day().At("10:00").Run(publishJob)
	scheduler.Every().Day().At("19:00").Run(publishJob)

	b.bot.Handle("/start", b.HandleStart)
	b.bot.Handle("/queue", b.HandleQueue)
	b.bot.Handle("/draft", b.HandleDraft)
	b.bot.Handle("/publish", b.HandlePublish)
	b.bot.Handle(tb.OnText, b.HandleDefaultText)
	b.bot.Handle(&postReadyBtn, b.HandlePostReadyCallback)
	b.bot.Handle(&postDraftBtn, b.HandlePostDraftCallback)
	b.bot.Handle(&postEditTitleBtn, b.HandlePostEditTitleCallback)
	b.bot.Handle(&postEditCommentBtn, b.HandlePostEditCommentCallback)

	logrus.Info("Bot is ready")
	b.bot.Start()
	return nil
}

func (b *BotServer) HandleStart(msg *tb.Message) {
	b.bot.Send(msg.Sender, "Please paste a new url")
}

func (b *BotServer) HandlePublish(msg *tb.Message) {
	_ = b.PublishPosts(1)
	b.bot.Send(msg.Sender, "Published")
}

func (b *BotServer) HandleDraft(msg *tb.Message) {
	b.listPosts(msg, PostStatusDraft)
}

func (b *BotServer) HandleQueue(msg *tb.Message) {
	b.listPosts(msg, PostStatusReady)
}

func (b *BotServer) listPosts(msg *tb.Message, status PostStatus) {
	posts, count, err := b.postMgr.GetPosts(0, status)
	if err != nil {
		logrus.WithError(err).Warn("cannot load posts")
		return
	}
	b.bot.Send(msg.Sender, fmt.Sprintf("%d posts.", count))

	i := 1
	selected := []*Post{}
	for {
		if len(posts) > postListChunkSize {
			selected, posts = posts[:postListChunkSize], posts[postListChunkSize:]
		} else {
			selected, posts = posts, []*Post{}
		}
		if len(selected) == 0 {
			break
		}
		res := ""
		for _, p := range selected {
			res += fmt.Sprintf("%d. %s", i, p.MarkdownSummary())
		}
		b.bot.Send(msg.Sender, res, &tb.SendOptions{
			ParseMode:             tb.ModeMarkdown,
			DisableWebPagePreview: true,
			DisableNotification:   true,
		})
	}
}

func (b *BotServer) HandlePostReadyCallback(cb *tb.Callback) {
	cbData := getFromCallback(cb.Data)
	if cbData == nil {
		return
	}
	_ = b.postMgr.UpdatePostStatus(cbData.PostID, PostStatusDraft, PostStatusReady)
	b.bot.Send(cb.Sender, "Post status -> READY")

	newDetail, opt, err := b.getPostMessage(cbData.PostID)
	if err != nil {
		logrus.WithError(err).Warn("cannot load get post")
		return
	}
	_, _ = b.bot.Edit(cb.Message, newDetail, opt)

	b.bot.Respond(cb, &tb.CallbackResponse{})
}

func (b *BotServer) HandlePostDraftCallback(cb *tb.Callback) {
	cbData := getFromCallback(cb.Data)
	if cbData == nil {
		return
	}
	_ = b.postMgr.UpdatePostStatus(cbData.PostID, PostStatusReady, PostStatusDraft)
	b.bot.Send(cb.Sender, "Post status -> DRAFT")

	newDetail, opt, err := b.getPostMessage(cbData.PostID)
	if err != nil {
		logrus.WithError(err).Warn("cannot load get post")
		return
	}
	_, _ = b.bot.Edit(cb.Message, newDetail, opt)

	b.bot.Respond(cb, &tb.CallbackResponse{})
}

func (b *BotServer) HandlePostEditTitleCallback(cb *tb.Callback) {
	cbData := getFromCallback(cb.Data)
	if cbData == nil {
		return
	}
	b.bot.Send(cb.Sender, "Please enter title")
	ps := b.postMgr.GetState(cb.Sender.ID)
	ps.PostID = cbData.PostID
	ps.MessageID = cb.Message.ID
	ps.ChatID = cb.Message.Chat.ID
	ps.EditStatus = PostEditStatusEditingTitle
	b.postMgr.StoreState(cb.Sender.ID, ps)

	b.bot.Respond(cb, &tb.CallbackResponse{})
}

func (b *BotServer) HandlePostEditCommentCallback(cb *tb.Callback) {
	cbData := getFromCallback(cb.Data)
	if cbData == nil {
		return
	}
	b.bot.Send(cb.Sender, "Please enter comment")
	ps := b.postMgr.GetState(cb.Sender.ID)
	ps.PostID = cbData.PostID
	ps.MessageID = cb.Message.ID
	ps.ChatID = cb.Message.Chat.ID
	ps.EditStatus = PostEditStatusEditingComment
	b.postMgr.StoreState(cb.Sender.ID, ps)

	b.bot.Respond(cb, &tb.CallbackResponse{})
}

func getFromCallback(data string) *PostCallbackData {
	var cbData PostCallbackData
	if err := json.Unmarshal([]byte(data), &cbData); err != nil {
		return nil
	}
	return &cbData
}

func (b *BotServer) HandleURL(msg *tb.Message, url *url.URL) {
	adminID := msg.Sender.ID
	ps, err := b.postMgr.LoadOrStorePostState(adminID, url)
	if err != nil {
		logrus.WithError(err).Warn("cannot load post state")
		return
	}
	text, opt, err := b.getPostMessage(ps.PostID)
	if err != nil {
		logrus.WithError(err).Warn("cannot load get post")
		return
	}
	b.bot.Send(msg.Sender, text, opt)
}

func (b *BotServer) getPostMessage(postID uint) (text string, opt *tb.SendOptions, err error) {
	p, err := b.postMgr.GetPost(postID)
	if err != nil {
		return "", nil, err
	}
	cbData := &PostCallbackData{
		PostID: postID,
	}
	data, err := json.Marshal(cbData)
	if err != nil {
		return "", nil, err
	}
	postDetail := p.Markdown()

	kb := [][]tb.InlineButton{}
	switch p.Status {
	case PostStatusDraft:
		kb = append(kb,
			[]tb.InlineButton{
				tb.InlineButton{
					Unique: "ready",
					Text:   "Ready to publish",
					Data:   string(data),
				}},
		)
	case PostStatusReady:
		kb = append(kb,
			[]tb.InlineButton{
				tb.InlineButton{
					Unique: "draft",
					Text:   "Back to draft",
					Data:   string(data),
				}},
		)
	}
	kb = append(kb,
		[]tb.InlineButton{
			tb.InlineButton{
				Unique: "edit_title",
				Text:   "Edit title",
				Data:   string(data),
			}},
		[]tb.InlineButton{
			tb.InlineButton{
				Unique: "edit_comment",
				Text:   "Edit comment",
				Data:   string(data),
			}},
	)

	return postDetail, &tb.SendOptions{
		ParseMode: tb.ModeMarkdown,
		ReplyMarkup: &tb.ReplyMarkup{
			InlineKeyboard: kb,
		},
	}, nil
}

func (b *BotServer) HandleDefaultText(msg *tb.Message) {
	txt := msg.Text
	adminID := msg.Sender.ID
	ps := b.postMgr.GetState(adminID)
	u, err := url.ParseRequestURI(strings.TrimSpace(txt))
	if err == nil {
		b.HandleURL(msg, u)
		return
	}
	if ps.PostID > 0 {
		switch ps.EditStatus {
		case PostEditStatusNothing:
			b.bot.Send(msg.Sender, "Cannot recognize.")
		case PostEditStatusEditingTitle:
			_ = b.postMgr.UpdatePostTitle(ps.PostID, txt)
			ps.EditStatus = PostEditStatusNothing
			newDetail, opt, err := b.getPostMessage(ps.PostID)
			if err != nil {
				logrus.WithError(err).Warn("cannot load get post")
				return
			}
			b.postMgr.StoreState(adminID, ps)
			b.bot.Send(msg.Sender, "Post title is changed")
			b.bot.Edit(ps, newDetail, opt)
		case PostEditStatusEditingComment:
			_ = b.postMgr.UpdatePostComment(ps.PostID, txt)
			ps.EditStatus = PostEditStatusNothing
			newDetail, opt, err := b.getPostMessage(ps.PostID)
			if err != nil {
				logrus.WithError(err).Warn("cannot load get post")
				return
			}
			b.postMgr.StoreState(adminID, ps)
			b.bot.Send(msg.Sender, "Post comment is changed")
			b.bot.Edit(ps, newDetail, opt)
		}
	}
}

func (b *BotServer) PublishPosts(limit int) error {
	logrus.Printf("Publishing comment to %s", b.botChannel)
	ch := &tb.Chat{Username: b.botChannel, Type: tb.ChatChannel}

	posts, _, err := b.postMgr.GetPosts(limit, PostStatusReady)
	if err != nil {
		return err
	}
	for _, p := range posts {
		_, err := b.bot.Send(ch, p.MarkdownPublish(), &tb.SendOptions{
			ParseMode: tb.ModeMarkdown,
		})
		if err != nil {
			logrus.WithError(err).WithField("post_id", p.ID).Warn("cannot post")
		}
		b.postMgr.UpdatePostStatus(p.ID, PostStatusReady, PostStatusPublished)
	}
	return nil
}
