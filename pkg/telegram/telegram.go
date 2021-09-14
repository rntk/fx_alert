package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	apiURL = "https://api.telegram.org"
)

type UpdatesResponse struct {
	OK     bool
	Result []Update
}

type User struct {
	ID       int64
	Username string
}

type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  Message
}

type Message struct {
	MessageID int64 `json:"message_id"`
	Text      string
	From      User
	Chat      Chat
}

type Chat struct {
	ID int64
}

type Telegram struct {
	m               sync.Mutex
	token           string
	lastUpdateID    int64
	client          *http.Client
	longPollClient  *http.Client
	longPollTimeout uint
}

func (t *Telegram) GetUpdates(ctx context.Context, longPoll bool) ([]Update, error) {
	t.m.Lock()
	defer t.m.Unlock()
	upds, err := t.getUpdates(ctx, longPoll)
	if err != nil {
		return nil, fmt.Errorf("Can't get updates. Offset: %d. %w", t.lastUpdateID, err)
	}
	last := len(upds) - 1
	if last >= 0 {
		t.lastUpdateID = upds[last].UpdateID
	}

	return upds, nil
}

func (t *Telegram) getUpdates(ctx context.Context, longPoll bool) ([]Update, error) {
	client := t.client
	timeout := ""
	if longPoll {
		client = t.longPollClient
		timeout = "&timeout=" + strconv.FormatInt(int64(t.longPollTimeout), 10)
	}
	URL := fmt.Sprintf(
		"%s/bot%s/getUpdates?offset=%d%s",
		apiURL,
		t.token,
		t.lastUpdateID+1,
		timeout,
	)
	rq, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, fmt.Errorf("Can`t create request: %w", err)
	}
	resp, err := client.Do(rq.WithContext(ctx))
	if err != nil {
		client.CloseIdleConnections()
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var upds UpdatesResponse
	if err := json.Unmarshal(body, &upds); err != nil {
		return nil, fmt.Errorf("Can`t unmarshal: %q. %w", body, err)
	}
	if !upds.OK {
		return nil, fmt.Errorf("Response is not OK: %q", body)
	}

	return upds.Result, nil
}

func New(token string) *Telegram {
	longPollSeconds := 60
	return &Telegram{
		m:               sync.Mutex{},
		token:           token,
		longPollTimeout: uint(longPollSeconds),
		client:          &http.Client{Timeout: 5 * time.Second},
		longPollClient:  &http.Client{Timeout: time.Duration(longPollSeconds+5) * time.Second},
	}
}
