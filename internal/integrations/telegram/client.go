package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxResponse = 8 << 20

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}
type Options struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text,omitempty"`
}
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
type SendMessageRequest struct {
	ChatID                int64                 `json:"chat_id"`
	Text                  string                `json:"text"`
	ReplyMarkup           *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
	DisableWebPagePreview bool                  `json:"disable_web_page_preview,omitempty"`
}

type APIError struct {
	Code        int
	Description string
	RetryAfter  int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Telegram Bot API error %d: %s", e.Code, e.Description)
}

func New(opts Options) (*Client, error) {
	token := strings.TrimSpace(opts.Token)
	if token == "" || strings.ContainsAny(token, "/\\\r\n ") {
		return nil, errors.New("Telegram bot token invalid")
	}
	raw := strings.TrimSpace(opts.BaseURL)
	if raw == "" {
		raw = "https://api.telegram.org"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("Telegram Bot API base URL invalid")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, errors.New("Telegram Bot API URL must use http or https")
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 65 * time.Second}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return &Client{base: u, token: token, http: hc}, nil
}
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var out User
	return out, c.call(ctx, "getMe", nil, &out)
}
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	if timeout < 0 || timeout > 50 {
		timeout = 30
	}
	var out []Update
	return out, c.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": timeout, "allowed_updates": []string{"message", "callback_query"}}, &out)
}
func (c *Client) SendMessage(ctx context.Context, in SendMessageRequest) (Message, error) {
	if in.ChatID == 0 || strings.TrimSpace(in.Text) == "" {
		return Message{}, errors.New("Telegram chat id and text required")
	}
	var out Message
	return out, c.call(ctx, "sendMessage", in, &out)
}
func (c *Client) AnswerCallbackQuery(ctx context.Context, id, text string) error {
	if id == "" {
		return errors.New("callback query id required")
	}
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}
func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	if c == nil || c.base == nil {
		return errors.New("Telegram client unavailable")
	}
	if method == "" || strings.ContainsAny(method, "/\\.? ") {
		return errors.New("Telegram method invalid")
	}
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + "/bot" + c.token + "/" + method
	var r io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// The request URL contains /bot<TOKEN>/; never propagate transport
		// error text because transports may include that URL.
		return errors.New("Telegram Bot API transport error")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponse {
		return errors.New("Telegram Bot API response too large")
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode Telegram Bot API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.OK {
		return &APIError{Code: envelope.ErrorCode, Description: envelope.Description, RetryAfter: envelope.Parameters.RetryAfter}
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return err
		}
	}
	return nil
}
func ChatIDString(id int64) string { return strconv.FormatInt(id, 10) }
