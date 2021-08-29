package telegram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	token        string
	updatesCh    chan Message
	stopCh       chan struct{}
	lastUpdateID int64
	client       *http.Client
	errCh        chan error
}

func (t *Telegram) Start(pause time.Duration) <-chan Message {
	if t.updatesCh != nil {
		return t.updatesCh
	}
	t.updatesCh = make(chan Message)
	t.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(pause)
		defer func() {
			ticker.Stop()
		}()
		for {
			select {
			case <-t.stopCh:
				t.stop()
				return
			case <-ticker.C:
				upds, err := t.getUpdates()
				if err != nil {
					t.errCh <- fmt.Errorf("Can't get updates. Offset: %d. %w", t.lastUpdateID, err)
					upds = nil
				}
				for _, upd := range upds {
				msgLoop:
					for {
						select {
						case t.updatesCh <- upd.Message:
							t.lastUpdateID = upd.UpdateID
							break msgLoop
						case <-t.stopCh:
							t.stop()
							return
						}
					}
				}
			}
		}
	}()

	return t.updatesCh
}

func (t *Telegram) Stop() {
	if t.stopCh == nil {
		return
	}
	t.stopCh <- struct{}{}
}

func (t *Telegram) stop() {
	close(t.updatesCh)
	t.updatesCh = nil
	close(t.stopCh)
	t.stopCh = nil
}

func (t *Telegram) getUpdates() ([]Update, error) {
	resp, err := t.client.Get(
		fmt.Sprintf(
			"%s/bot%s/getUpdates?offset=%d",
			apiURL,
			t.token,
			t.lastUpdateID+1,
		),
	)
	if err != nil {
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

func (t *Telegram) Errors() <-chan error {
	return t.errCh
}

func New(token string) *Telegram {
	return &Telegram{
		token:  token,
		client: &http.Client{Timeout: 5 * time.Second},
		errCh:  make(chan error),
	}
}
