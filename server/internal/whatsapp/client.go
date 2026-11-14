package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"louvadeira/internal/messenger"
	"louvadeira/internal/storage"
)

const connectTimeout = 30 * time.Second

type Status struct {
	Linked    bool `json:"linked"`
	Connected bool `json:"connected"`
}

type Group struct {
	ID   messenger.Address `json:"id"`
	Name string            `json:"name"`
}

type Participant struct {
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

/**
 * NOTE: Client links the server to a WhatsApp number as a companion device, like WhatsApp Web. It only acts on
 * what the app itself sent: votes on its own polls (handed to the vote handler) and replies to its button tests.
 * Every other chat on the number is ignored and never stored.
 */
type Client struct {
	wm        *whatsmeow.Client
	container *sqlstore.Container
	log       *slog.Logger

	connected chan struct{}
	once      sync.Once

	mu          sync.Mutex
	onVote      func(context.Context, messenger.Vote)
	buttonTests map[types.MessageID]string
}

func Open(ctx context.Context, dbPath string, log *slog.Logger) (*Client, error) {
	store.SetOSInfo("louvadeira", [3]uint32{0, 1, 0})
	container, err := sqlstore.New(ctx, "sqlite", storage.DSN(dbPath), waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("abrir sessão do WhatsApp: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		container.Close()
		return nil, fmt.Errorf("ler aparelho do WhatsApp: %w", err)
	}
	wm := whatsmeow.NewClient(device, waLog.Stdout("whatsmeow", "WARN", false))
	// NOTE: Manual mode means the chat history WhatsApp offers on linking is never downloaded.
	wm.ManualHistorySyncDownload = true

	c := &Client{
		wm:          wm,
		container:   container,
		log:         log,
		connected:   make(chan struct{}),
		buttonTests: make(map[types.MessageID]string),
	}
	wm.AddEventHandler(c.handle)
	return c, nil
}

func (c *Client) Close() {
	c.wm.Disconnect()
	c.container.Close()
}

func (c *Client) OnVote(handler func(context.Context, messenger.Vote)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onVote = handler
}

func (c *Client) OwnPhone() string {
	if c.wm.Store.ID == nil {
		return ""
	}
	return c.wm.Store.ID.User
}

func (c *Client) Status() Status {
	return Status{Linked: c.wm.Store.ID != nil, Connected: c.wm.IsConnected() && c.wm.IsLoggedIn()}
}

func (c *Client) Connect(ctx context.Context) error {
	if c.wm.Store.ID == nil {
		return errors.New("WhatsApp não vinculado; rode 'louvadeira whatsapp link'")
	}
	if err := c.wm.Connect(); err != nil {
		return fmt.Errorf("conectar ao WhatsApp: %w", err)
	}
	select {
	case <-c.connected:
		return nil
	case <-time.After(connectTimeout):
		return errors.New("WhatsApp não confirmou a conexão a tempo")
	case <-ctx.Done():
		return ctx.Err()
	}
}

/**
 * NOTE: With an empty phone the link uses a QR code printed to out; with a phone number (country code
 * included) it prints an 8-character code to type under "Conectar com número de telefone".
 */
func (c *Client) Link(ctx context.Context, phone string, out io.Writer) error {
	if c.wm.Store.ID != nil {
		return errors.New("este servidor já está vinculado; rode 'louvadeira whatsapp logout' antes")
	}
	qrs, err := c.wm.GetQRChannel(ctx)
	if err != nil {
		return err
	}
	if err := c.wm.Connect(); err != nil {
		return fmt.Errorf("conectar ao WhatsApp: %w", err)
	}
	codeShown := false
	for item := range qrs {
		switch {
		case item.Event == whatsmeow.QRChannelEventCode && phone == "":
			fmt.Fprintln(out, "\nNo celular: WhatsApp > Aparelhos conectados > Conectar aparelho, e leia o QR Code:")
			qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, out)
		case item.Event == whatsmeow.QRChannelEventCode && !codeShown:
			code, err := c.wm.PairPhone(ctx, string(messenger.Address(phone).Normalize()), true, whatsmeow.PairClientChrome, "Chrome (Linux)")
			if err != nil {
				return fmt.Errorf("gerar código de pareamento: %w", err)
			}
			fmt.Fprintf(out, "\nNo celular: WhatsApp > Aparelhos conectados > Conectar aparelho > Conectar com número de telefone.\nDigite o código: %s\n", code)
			codeShown = true
		case item == whatsmeow.QRChannelSuccess:
			fmt.Fprintln(out, "Vinculado. Terminando a configuração...")
			return c.waitConnected(ctx)
		case item == whatsmeow.QRChannelTimeout:
			return errors.New("tempo esgotado; rode o comando de novo")
		case item.Event == whatsmeow.QRChannelEventError:
			return fmt.Errorf("vínculo falhou: %w", item.Error)
		case item.Event != whatsmeow.QRChannelEventCode:
			return fmt.Errorf("vínculo falhou: %s", item.Event)
		}
	}
	return errors.New("o WhatsApp encerrou o vínculo sem confirmar")
}

func (c *Client) Logout(ctx context.Context) error {
	if c.wm.Store.ID == nil {
		return nil
	}
	if !c.wm.IsConnected() {
		if err := c.Connect(ctx); err != nil {
			return err
		}
	}
	return c.wm.Logout(ctx)
}

func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	infos, err := c.wm.GetJoinedGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar grupos: %w", err)
	}
	groups := make([]Group, 0, len(infos))
	for _, info := range infos {
		groups = append(groups, Group{ID: messenger.Address(info.JID.String()), Name: info.Name})
	}
	return groups, nil
}

/**
 * NOTE: Names come from the linked phone's contacts or, failing that, the participant's WhatsApp push name, so the
 * admin can recognise who is who. Members hidden behind a LID without a known phone number are left out.
 */
func (c *Client) GroupParticipants(ctx context.Context, groupID messenger.Address) ([]Participant, error) {
	jid, err := types.ParseJID(string(groupID.Normalize()))
	if err != nil || jid.Server != types.GroupServer {
		return nil, fmt.Errorf("grupo inválido: %s", groupID)
	}
	info, err := c.wm.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("ler o grupo: %w", err)
	}
	participants := make([]Participant, 0, len(info.Participants))
	for _, member := range info.Participants {
		phone := member.PhoneNumber
		if phone.IsEmpty() && member.JID.Server == types.DefaultUserServer {
			phone = member.JID
		}
		if phone.IsEmpty() && member.JID.Server == types.HiddenUserServer {
			phone, _ = c.wm.Store.LIDs.GetPNForLID(ctx, member.JID)
		}
		if phone.IsEmpty() {
			continue
		}
		participants = append(participants, Participant{Phone: phone.User, Name: c.contactName(ctx, phone, member.JID)})
	}
	return participants, nil
}

func (c *Client) contactName(ctx context.Context, ids ...types.JID) string {
	for _, id := range ids {
		contact, err := c.wm.Store.Contacts.GetContact(ctx, id)
		if err != nil || !contact.Found {
			continue
		}
		for _, name := range []string{contact.FullName, contact.FirstName, contact.PushName, contact.BusinessName} {
			if name != "" {
				return name
			}
		}
	}
	return ""
}

func (c *Client) SendText(ctx context.Context, to messenger.Address, text string) (messenger.MessageID, error) {
	jid, err := c.resolve(ctx, to)
	if err != nil {
		return "", err
	}
	resp, err := c.wm.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		return "", fmt.Errorf("enviar mensagem: %w", err)
	}
	return messenger.MessageID(resp.ID), nil
}

func (c *Client) SendPoll(ctx context.Context, to messenger.Address, poll messenger.Poll) (messenger.MessageID, error) {
	jid, err := c.resolve(ctx, to)
	if err != nil {
		return "", err
	}
	// NOTE: WhatsApp reads a selectable count of 0 as "any number of options".
	selectable := 1
	if poll.Multiple {
		selectable = 0
	}
	resp, err := c.wm.SendMessage(ctx, jid, c.wm.BuildPollCreation(poll.Question, poll.Options, selectable))
	if err != nil {
		return "", fmt.Errorf("enviar enquete: %w", err)
	}
	return messenger.MessageID(resp.ID), nil
}

/**
 * WARNING: Brazilian accounts created before the ninth digit may be registered without it, so a phone number
 * cannot be turned into a chat ID by hand. WhatsApp's lookup returns the ID the account really uses.
 */
func (c *Client) resolve(ctx context.Context, to messenger.Address) (types.JID, error) {
	normalized := to.Normalize()
	if normalized == "" {
		return types.JID{}, errors.New("destino vazio")
	}
	if strings.Contains(string(normalized), "@") {
		return types.ParseJID(string(normalized))
	}
	found, err := c.wm.IsOnWhatsApp(ctx, []string{"+" + string(normalized)})
	if err != nil {
		return types.JID{}, fmt.Errorf("procurar %s no WhatsApp: %w", normalized, err)
	}
	if len(found) == 0 || !found[0].IsIn {
		return types.JID{}, fmt.Errorf("%s não tem WhatsApp", normalized)
	}
	return found[0].JID, nil
}

func (c *Client) handle(evt any) {
	switch e := evt.(type) {
	case *events.Connected:
		c.once.Do(func() { close(c.connected) })
		c.log.Info("WhatsApp conectado")
	case *events.LoggedOut:
		c.log.Warn("WhatsApp desvinculado pelo celular; rode 'louvadeira whatsapp link' de novo", "motivo", e.Reason.String())
	case *events.Message:
		c.handleMessage(e)
	}
}

func (c *Client) handleMessage(e *events.Message) {
	if update := e.Message.GetPollUpdateMessage(); update != nil {
		c.handleVote(e)
		return
	}
	c.logButtonReply(e)
}

/**
 * NOTE: Decryption only works for polls this number created, so votes on anyone else's polls fail here and are
 * dropped without being read.
 */
func (c *Client) handleVote(e *events.Message) {
	ctx := context.Background()
	vote, err := c.wm.DecryptPollVote(ctx, e)
	if err != nil {
		return
	}
	c.mu.Lock()
	handler := c.onVote
	c.mu.Unlock()
	if handler == nil {
		return
	}
	handler(ctx, messenger.Vote{
		PollID:   messenger.MessageID(e.Message.GetPollUpdateMessage().GetPollCreationMessageKey().GetID()),
		Selected: vote.GetSelectedOptions(),
		FromMe:   e.Info.IsFromMe,
		Sender:   e.Info.PushName,
	})
}

func (c *Client) waitConnected(ctx context.Context) error {
	select {
	case <-c.connected:
		return nil
	case <-time.After(connectTimeout):
		return errors.New("vinculado, mas a conexão não terminou a tempo; reinicie o serviço")
	case <-ctx.Done():
		return ctx.Err()
	}
}
