package main

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/goware/urlx"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type PostStatus int
type PostEditStatus int

const (
	PostStatusDraft PostStatus = iota + 1
	PostStatusReady
	PostStatusPublished
)

const (
	PostEditStatusNothing PostEditStatus = iota
	PostEditStatusEditingTitle
	PostEditStatusEditingComment
)

var (
	ErrPostExists   = errors.New("post url exists")
	ErrPostNotFound = errors.New("post not found")
)

type Post struct {
	gorm.Model
	URL     string `gorm:"size:255;unique_index;not null"`
	Status  PostStatus
	Title   string
	Comment string
}

func (p *Post) Markdown() string {
	res := ""
	res += "*URL*\n"
	res += p.URL + "\n\n"
	res += "*Title*\n"
	if p.Title != "" {
		res += p.Title + "\n\n"
	} else {
		res += "{EMPTY}\n\n"
	}
	res += "*Comment*\n"
	if p.Comment != "" {
		res += p.Comment + "\n\n"
	} else {
		res += "{EMPTY}\n\n"
	}

	switch p.Status {
	case PostStatusDraft:
		res += "*Status* DRAFT\n"
	case PostStatusReady:
		res += "*Status* READY\n"
	case PostStatusPublished:
		res += "*Status* PUBLISHED\n"
	}
	res += fmt.Sprintf("*CreatedAt* %s\n", p.CreatedAt.Format(time.RFC1123))
	res += fmt.Sprintf("*UpdatedAt* %s\n", p.UpdatedAt.Format(time.RFC1123))
	return res
}

func (p *Post) MarkdownPublish() string {
	res := ""
	if p.Title != "" {
		res += fmt.Sprintf("*%s*\n", p.Title)
	}
	res += p.URL + "\n\n"
	if p.Comment != "" {
		res += p.Comment + "\n"
	}
	return res
}

func (p *Post) MarkdownSummary() string {
	return fmt.Sprintf("*%s*\n%s\n", p.Title, p.URL)
}

type PostManager struct {
	db     *gorm.DB
	states *sync.Map // map<admin_user_ID:string, PostState>
}

type PostState struct {
	PostID     uint
	EditStatus PostEditStatus
	MessageID  int
	ChatID     int64
	*sync.RWMutex
}

func (ps *PostState) MessageSig() (string, int64) {
	return strconv.Itoa(ps.MessageID), ps.ChatID
}

func NewPostManager(db *gorm.DB) PostManager {
	return PostManager{
		db:     db,
		states: new(sync.Map),
	}
}

func normalizeURL(url *url.URL) (string, error) {
	return urlx.Normalize(url)
}

func (pm *PostManager) GetState(adminID int) *PostState {
	ps := PostState{RWMutex: new(sync.RWMutex)}
	actual, loaded := pm.states.LoadOrStore(adminID, &ps)
	if loaded {
		return actual.(*PostState)
	}
	return &ps
}

func (pm *PostManager) StoreState(adminID int, ps *PostState) {
	pm.states.Store(adminID, ps)
}

func (pm *PostManager) LoadOrStorePostState(adminID int, url *url.URL) (*PostState, error) {
	post, err := pm.GetPostByURL(url)
	if err == ErrPostNotFound {
		newURL, err := normalizeURL(url)
		if err != nil {
			return nil, err
		}
		post = &Post{URL: newURL, Status: PostStatusDraft}
		if err = pm.db.Create(post).Error; err != nil {
			return nil, err
		}
	}
	ps := pm.GetState(adminID)
	ps.PostID = post.ID
	ps.EditStatus = PostEditStatusNothing
	pm.StoreState(adminID, ps)
	return ps, nil
}

func (pm *PostManager) GetPost(id uint) (*Post, error) {
	var p Post
	if err := pm.db.First(&p, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (pm *PostManager) UpdatePostTitle(id uint, title string) error {
	var post Post
	return pm.db.Model(&post).
		Where("id = ? AND status IN (?)",
			id, []PostStatus{PostStatusDraft, PostStatusReady}).
		Update("title", title).Error
}

func (pm *PostManager) UpdatePostComment(id uint, comment string) error {
	var post Post
	return pm.db.Model(&post).
		Where("id = ? AND status IN (?)",
			id, []PostStatus{PostStatusDraft, PostStatusReady}).
		Update("comment", comment).Error
}

func (pm *PostManager) UpdatePostStatus(id uint, from, to PostStatus) error {
	var post Post
	return pm.db.Model(&post).
		Where("id = ? AND status = ?",
			id, from).
		Update("status", to).Error
}

func (pm *PostManager) GetPostByURL(url *url.URL) (*Post, error) {
	n, err := normalizeURL(url)
	if err != nil {
		return nil, err
	}
	var p Post
	err = pm.db.First(&p, "url = ?", n).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (pm *PostManager) GetPosts(limit int, status PostStatus) (posts []*Post, count int, err error) {
	q := pm.db.Where("status = ?", status)
	if limit > 0 {
		q = q.Limit(limit)
	}
	err = q.Find(&posts).Count(&count).Error
	return
}
