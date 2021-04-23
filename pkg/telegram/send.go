package telegram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
)

type sendMessageResponse struct {
	OK bool
}

func (t *Telegram) SendMessage(chatID int64, msg string, msgID int64) error {
	replyTo := ""
	if msgID > 0 {
		replyTo = "&reply_to_message_id=" + strconv.FormatInt(msgID, 10)
	}
	resp, err := t.client.Get(
		fmt.Sprintf(
			"%s/bot%s/sendMessage?chat_id=%d&text=%s%s",
			apiURL,
			t.token,
			chatID,
			url.QueryEscape(msg),
			replyTo,
		),
	)
	if err != nil {
		return fmt.Errorf("Can't send message: %w", err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Can't read body: %v", err)
	}
	var smResp sendMessageResponse
	if err := json.Unmarshal(b, &smResp); err != nil {
		return fmt.Errorf("Can't unmarshal body: %q. %v", b, err)
	}
	if !smResp.OK {
		return fmt.Errorf("Can't send message: respons is not OK: %q", b)
	}

	return nil
}
