package telegram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
)

type Answer struct {
	Text          string
	ReplyKeyboard *ReplyKeyboardMarkup
}

type sendMessageResponse struct {
	OK bool
}

type ReplyKeyboardMarkup struct {
	Keyboard        [][]KeyboardButton `json:"keyboard"`
	OneTimeKeyboard bool               `json:"one_time_keyboard"`
}

type KeyboardButton struct {
	Text string `json:"text"`
}

func (t *Telegram) SendMessage(chatID int64, msgID int64, answer Answer) error {
	form := url.Values{}
	if msgID > 0 {
		form.Add("reply_to_message_id", strconv.FormatInt(msgID, 10))
	}
	if answer.ReplyKeyboard != nil {
		mb, err := json.Marshal(answer.ReplyKeyboard)
		if err != nil {
			return fmt.Errorf("Can't marshal markup: %w", err)
		}
		form.Add("reply_markup", string(mb))
	}
	form.Add("chat_id", strconv.FormatInt(chatID, 10))
	form.Add("text", answer.Text)
	resp, err := t.client.PostForm(
		fmt.Sprintf("%s/bot%s/sendMessage", apiURL, t.token),
		form,
	)
	if err != nil {
		t.client.CloseIdleConnections()
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
