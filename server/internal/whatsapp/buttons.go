package whatsapp

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"louvadeira/internal/messenger"
)

type ButtonTestResult struct {
	Kind  string `json:"kind"`
	Sent  bool   `json:"sent"`
	Error string `json:"error,omitempty"`
}

/**
 * NOTE: Sends the three kinds of interactive message WhatsApp has had (classic buttons, list, native-flow quick
 * replies) to see which ones a linked-device client can still show. Replies to them are logged.
 */
func (c *Client) SendButtonTests(ctx context.Context, to messenger.Address) ([]ButtonTestResult, error) {
	jid, err := c.resolve(ctx, to)
	if err != nil {
		return nil, err
	}
	tests := []struct {
		kind    string
		message *waE2E.Message
	}{
		{"botões clássicos", &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{
			ContentText: proto.String("Teste 1 de 3 (botões clássicos): você consegue tocar num botão?"),
			FooterText:  proto.String("louvadeira"),
			HeaderType:  waE2E.ButtonsMessage_EMPTY.Enum(),
			Buttons: []*waE2E.ButtonsMessage_Button{
				{ButtonID: proto.String("sim"), ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("✅ Sim")}, Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum()},
				{ButtonID: proto.String("nao"), ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("❌ Não")}, Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum()},
			},
		}}},
		{"lista", &waE2E.Message{ListMessage: &waE2E.ListMessage{
			Title:       proto.String("Teste 2 de 3 (lista)"),
			Description: proto.String("Abra a lista e escolha uma opção."),
			ButtonText:  proto.String("Responder"),
			ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
			FooterText:  proto.String("louvadeira"),
			Sections: []*waE2E.ListMessage_Section{{
				Title: proto.String("Presença"),
				Rows: []*waE2E.ListMessage_Row{
					{RowID: proto.String("sim"), Title: proto.String("✅ Confirmo")},
					{RowID: proto.String("nao"), Title: proto.String("❌ Não posso")},
				},
			}},
		}}},
		{"respostas rápidas (native flow)", &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{
			MessageContextInfo: &waE2E.MessageContextInfo{DeviceListMetadataVersion: proto.Int32(2)},
			InteractiveMessage: &waE2E.InteractiveMessage{
				Body:   &waE2E.InteractiveMessage_Body{Text: proto.String("Teste 3 de 3 (respostas rápidas): você consegue tocar num botão?")},
				Footer: &waE2E.InteractiveMessage_Footer{Text: proto.String("louvadeira")},
				InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
					MessageVersion: proto.Int32(1),
					Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
						{Name: proto.String("quick_reply"), ButtonParamsJSON: proto.String(`{"display_text":"✅ Sim","id":"sim"}`)},
						{Name: proto.String("quick_reply"), ButtonParamsJSON: proto.String(`{"display_text":"❌ Não","id":"nao"}`)},
					},
				}},
			},
		}}}},
	}
	results := make([]ButtonTestResult, 0, len(tests))
	for _, test := range tests {
		resp, err := c.wm.SendMessage(ctx, jid, test.message)
		result := ButtonTestResult{Kind: test.kind, Sent: err == nil}
		if err != nil {
			result.Error = err.Error()
		} else {
			c.mu.Lock()
			c.buttonTests[resp.ID] = test.kind
			c.mu.Unlock()
		}
		results = append(results, result)
	}
	return results, nil
}

func (c *Client) logButtonReply(e *events.Message) {
	m := e.Message
	var kind, choice, original string
	switch {
	case m.GetButtonsResponseMessage() != nil:
		r := m.GetButtonsResponseMessage()
		kind, choice, original = "botões clássicos", r.GetSelectedButtonID(), r.GetContextInfo().GetStanzaID()
	case m.GetListResponseMessage() != nil:
		r := m.GetListResponseMessage()
		kind, choice, original = "lista", r.GetSingleSelectReply().GetSelectedRowID(), r.GetContextInfo().GetStanzaID()
	case m.GetInteractiveResponseMessage() != nil:
		r := m.GetInteractiveResponseMessage()
		kind, choice, original = "respostas rápidas", r.GetNativeFlowResponseMessage().GetParamsJSON(), r.GetContextInfo().GetStanzaID()
	case m.GetTemplateButtonReplyMessage() != nil:
		r := m.GetTemplateButtonReplyMessage()
		kind, choice, original = "template", r.GetSelectedID(), r.GetContextInfo().GetStanzaID()
	default:
		return
	}
	c.mu.Lock()
	test, ours := c.buttonTests[original]
	c.mu.Unlock()
	if ours {
		c.log.Info("resposta ao teste de botões", "teste", test, "tipo da resposta", kind, "escolha", choice, "quem", e.Info.PushName)
	}
}
