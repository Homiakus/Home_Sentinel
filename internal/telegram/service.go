package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/auth"
	"github.com/Homiakus/Home_Sentinel/internal/authz"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"github.com/Homiakus/Home_Sentinel/internal/events"
	tgapi "github.com/Homiakus/Home_Sentinel/internal/integrations/telegram"
	"github.com/Homiakus/Home_Sentinel/internal/intercom"
)

type UserLookup interface {
	GetByID(context.Context, string) (auth.User, error)
}

type Service struct {
	Client   *tgapi.Client
	Pairings PairingStore
	Actions  ActionStore
	Users    UserLookup
	Intercom *intercom.Service
	Events   *events.Bus
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func (s *Service) Start(parent context.Context) error {
	if s == nil || s.Client == nil {
		return errors.New("Telegram client unavailable")
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	go s.poll(ctx)
	if s.Events != nil {
		s.wg.Add(1)
		go s.eventWorker(ctx)
	}
	return nil
}
func (s *Service) Close() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
	s.cancel = nil
}
func (s *Service) poll(ctx context.Context) {
	defer s.wg.Done()
	var offset int64
	backoff := time.Second
	for ctx.Err() == nil {
		updates, err := s.Client.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			delay := backoff
			var apiErr *tgapi.APIError
			if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
				delay = time.Duration(apiErr.RetryAfter) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
			continue
		}
		backoff = time.Second
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			_ = s.ProcessUpdate(ctx, u)
		}
	}
}

func (s *Service) eventWorker(ctx context.Context) {
	defer s.wg.Done()
	sub := s.Events.Subscribe(64)
	defer sub.Cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			_ = s.NotifyEvent(ctx, e)
		}
	}
}

func (s *Service) NotifyEvent(ctx context.Context, e events.Envelope) error {
	if e.Type != "intercom.button.pressed" {
		return nil
	}
	var payload struct {
		DeviceID string `json:"device_id"`
		Location string `json:"location"`
	}
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		return err
	}
	if payload.DeviceID == "" {
		return errors.New("doorbell event lacks device id")
	}
	bindings, err := s.Pairings.ListBindings(ctx)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		user, err := s.Users.GetByID(ctx, b.UserID)
		if err != nil || user.Disabled || !authz.Allowed(user.Role, authz.ViewSystem) {
			continue
		}
		text := "🔔 Звонок в домофон"
		if payload.Location != "" {
			text += "\nМесто: " + payload.Location
		}
		req := tgapi.SendMessageRequest{ChatID: b.ChatID, Text: text}
		if authz.Allowed(user.Role, authz.UnlockDoor) {
			corr, corrErr := domain.NewID("cor")
			if corrErr == nil {
				pending, actionErr := s.Actions.Create(ctx, b, "door.unlock.prepare", payload.DeviceID, corr, 10*time.Minute)
				if actionErr == nil {
					req.ReplyMarkup = &tgapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgapi.InlineKeyboardButton{{{Text: "🚪 Открыть", CallbackData: "prepare:" + pending.Token}}}}
				}
			}
		}
		_, _ = s.Client.SendMessage(ctx, req)
	}
	return nil
}

func (s *Service) ProcessUpdate(ctx context.Context, u tgapi.Update) error {
	if u.Message != nil && u.Message.From != nil {
		return s.processMessage(ctx, *u.Message)
	}
	if u.CallbackQuery != nil {
		return s.processCallback(ctx, *u.CallbackQuery)
	}
	return nil
}
func (s *Service) processMessage(ctx context.Context, m tgapi.Message) error {
	text := strings.TrimSpace(m.Text)
	if strings.HasPrefix(text, "/start ") {
		code := strings.TrimSpace(strings.TrimPrefix(text, "/start "))
		binding, err := s.Pairings.Consume(ctx, code, m.From.ID, m.Chat.ID)
		if err != nil {
			_, _ = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: m.Chat.ID, Text: "Код привязки недействителен или истёк."})
			return err
		}
		_, err = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: m.Chat.ID, Text: "Home Sentinel подключён. Аккаунт: " + binding.UserID})
		return err
	}
	binding, err := s.Pairings.Binding(ctx, m.From.ID)
	if err != nil {
		_, _ = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: m.Chat.ID, Text: "Telegram не привязан к Home Sentinel. Создайте код привязки в Dashboard."})
		return err
	}
	user, err := s.Users.GetByID(ctx, binding.UserID)
	if err != nil {
		return err
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case "/status":
		if !authz.Allowed(user.Role, authz.ViewSystem) {
			return errors.New("forbidden")
		}
		_, err = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: m.Chat.ID, Text: "Home Sentinel: online"})
		return err
	case "/door":
		if !authz.Allowed(user.Role, authz.ViewSystem) {
			return errors.New("forbidden")
		}
		_, err = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: m.Chat.ID, Text: "Используйте уведомление/кнопку конкретного домофона для запроса открытия. Команда требует отдельного подтверждения."})
		return err
	default:
		_, err = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: m.Chat.ID, Text: "Команды: /status, /door, /help"})
		return err
	}
}
func (s *Service) processCallback(ctx context.Context, q tgapi.CallbackQuery) error {
	if strings.HasPrefix(q.Data, "prepare:") {
		token := strings.TrimPrefix(q.Data, "prepare:")
		pending, err := s.Actions.Consume(ctx, q.From.ID, token)
		if err != nil || pending.Action != "door.unlock.prepare" {
			_ = s.Client.AnswerCallbackQuery(ctx, q.ID, "Запрос истёк")
			if err != nil {
				return err
			}
			return errors.New("invalid prepare action")
		}
		keyboard, err := s.CreateUnlockConfirmation(ctx, q.From.ID, pending.Target)
		if err != nil {
			_ = s.Client.AnswerCallbackQuery(ctx, q.ID, "Недостаточно прав или устройство недоступно")
			return err
		}
		if q.Message == nil {
			return errors.New("Telegram callback has no message")
		}
		_, err = s.Client.SendMessage(ctx, tgapi.SendMessageRequest{ChatID: q.Message.Chat.ID, Text: "Открыть дверь через " + pending.Target + "?", ReplyMarkup: &keyboard})
		_ = s.Client.AnswerCallbackQuery(ctx, q.ID, "Требуется подтверждение")
		return err
	}
	if strings.HasPrefix(q.Data, "cancel:") {
		token := strings.TrimPrefix(q.Data, "cancel:")
		_, err := s.Actions.Consume(ctx, q.From.ID, token)
		if err != nil {
			_ = s.Client.AnswerCallbackQuery(ctx, q.ID, "Действие уже истекло")
			return err
		}
		return s.Client.AnswerCallbackQuery(ctx, q.ID, "Отменено")
	}
	if !strings.HasPrefix(q.Data, "confirm:") {
		return s.Client.AnswerCallbackQuery(ctx, q.ID, "Неизвестное действие")
	}
	token := strings.TrimPrefix(q.Data, "confirm:")
	pending, err := s.Actions.Consume(ctx, q.From.ID, token)
	if err != nil {
		_ = s.Client.AnswerCallbackQuery(ctx, q.ID, "Действие истекло или уже использовано")
		return err
	}
	binding, err := s.Pairings.Binding(ctx, q.From.ID)
	if err != nil {
		return err
	}
	if binding.UserID != pending.UserID {
		return errors.New("Telegram binding changed since action creation")
	}
	user, err := s.Users.GetByID(ctx, binding.UserID)
	if err != nil {
		return err
	}
	switch pending.Action {
	case "door.unlock":
		if !authz.Allowed(user.Role, authz.UnlockDoor) {
			return errors.New("forbidden")
		}
		corr := domain.ID(pending.CorrelationID)
		_, err = s.Intercom.Unlock(ctx, intercom.UnlockRequest{DeviceID: pending.Target, ActorID: user.ID, CorrelationID: corr.String(), TTL: 5 * time.Second})
		if err != nil {
			_ = s.Client.AnswerCallbackQuery(ctx, q.ID, "Не удалось отправить команду")
			return err
		}
		return s.Client.AnswerCallbackQuery(ctx, q.ID, "Команда открытия отправлена")
	default:
		return fmt.Errorf("unsupported Telegram action %q", pending.Action)
	}
}
func (s *Service) CreateUnlockConfirmation(ctx context.Context, telegramUserID int64, deviceID string) (tgapi.InlineKeyboardMarkup, error) {
	binding, err := s.Pairings.Binding(ctx, telegramUserID)
	if err != nil {
		return tgapi.InlineKeyboardMarkup{}, err
	}
	user, err := s.Users.GetByID(ctx, binding.UserID)
	if err != nil {
		return tgapi.InlineKeyboardMarkup{}, err
	}
	if !authz.Allowed(user.Role, authz.UnlockDoor) {
		return tgapi.InlineKeyboardMarkup{}, errors.New("forbidden")
	}
	corr, err := domain.NewID("cor")
	if err != nil {
		return tgapi.InlineKeyboardMarkup{}, err
	}
	pending, err := s.Actions.Create(ctx, binding, "door.unlock", deviceID, corr, 90*time.Second)
	if err != nil {
		return tgapi.InlineKeyboardMarkup{}, err
	}
	return tgapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgapi.InlineKeyboardButton{{{Text: "Да, открыть", CallbackData: "confirm:" + pending.Token}, {Text: "Отмена", CallbackData: "cancel:" + pending.Token}}}}, nil
}
