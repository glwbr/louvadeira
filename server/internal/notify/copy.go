package notify

import (
	"fmt"
	"strings"

	"louvadeira/internal/store"
)

/**
 * NOTE: Every message the team reads is written here, in a warm and plain voice: no "votar", no "vaga", no orders.
 * WhatsApp answers a poll with a hash of the chosen label and each poll keeps the labels it was sent with, so
 * rewording an option only changes polls sent afterwards.
 */

const (
	optionGoing       = "✅ Vou estar lá"
	optionGoingBoth   = "✅ Vou nos dois"
	optionGoingAll    = "✅ Vou em todos"
	optionNotGoing    = "😕 Não vou conseguir"
	optionNotGoingAny = "😕 Não consigo em nenhum"
	optionCanCover    = "✅ Consigo"
	optionCannotCover = "😕 Dessa vez não"
	optionCoverNone   = "😕 Nenhum dessa vez"
)

func goingOption(services int) string {
	switch services {
	case 1:
		return optionGoing
	case 2:
		return optionGoingBoth
	}
	return optionGoingAll
}

func notGoingOption(date string) string {
	return fmt.Sprintf("😕 Não consigo %s (%s)", onWeekday(date), dayMonth(date))
}

func confirmLine(o occurrence) string {
	return fmt.Sprintf("• %s, %s, às %s (%s)%s", capitalize(weekday(o.date)), dayMonth(o.date), hour(o.startTime),
		joinNames(o.roles), communionMark(o.communion))
}

func confirmQuestion(name, ministry string, lines []string) string {
	return fmt.Sprintf("Oi, %s! 😊 Você está na escala do %s:\n%s\nConsegue ir?", firstName(name), ministry, strings.Join(lines, "\n"))
}

func confirmedReply(name string) string {
	return fmt.Sprintf("Que bom, %s! Tá confirmado 🙌 Se surgir algum imprevisto, é só mudar sua resposta na enquete.", firstName(name))
}

func reminderText(d store.AssignmentDetail, roles []string) string {
	text := fmt.Sprintf("Oi, %s! Amanhã (%s, %s) tem culto às %s e você está no %s (%s) 🎶",
		firstName(d.PersonName), weekday(d.Date), dayMonth(d.Date), hour(d.StartTime), d.MinistryName, joinNames(roles))
	if d.Communion {
		text += " É dia de Santa Ceia 🍷"
	}
	return text + "\nSe surgir algum imprevisto, é só mudar sua resposta na enquete de confirmação."
}

func pendingAlert(d store.AssignmentDetail, names []string) string {
	return fmt.Sprintf("⏳ Ainda falta responder sobre %s (%s): %s.", when(d), d.MinistryName, joinNames(names))
}

func declinedReply(name string, invited []string) string {
	text := fmt.Sprintf("Tudo bem, %s, obrigado por avisar! 🙏 Já avisei a liderança", firstName(name))
	if len(invited) > 0 {
		text += " e chamei " + joinNames(invited) + " para ficar no seu lugar"
	}
	return text + "."
}

func coverQuestion(candidate string, d store.AssignmentDetail) string {
	return fmt.Sprintf("Oi, %s! %s não vai conseguir ir %s, %s, às %s (%s). Você consegue substituir?",
		firstName(candidate), d.PersonName, onWeekday(d.Date), dayMonth(d.Date), hour(d.StartTime), d.RoleName)
}

func coverManyQuestion(candidate string, d store.AssignmentDetail, period string) string {
	if period == "" {
		return fmt.Sprintf("Oi, %s! %s não vai conseguir ir nestes cultos do %s. Em quais você consegue substituir? Pode marcar mais de um 🙂",
			firstName(candidate), d.PersonName, d.MinistryName)
	}
	return fmt.Sprintf("Oi, %s! %s não vai poder servir no %s %s. Em quais destes cultos você consegue substituir? Pode marcar mais de um 🙂",
		firstName(candidate), d.PersonName, d.MinistryName, period)
}

func coverOption(label string) string {
	return "✅ " + capitalize(label)
}

func declinedAlert(d store.AssignmentDetail, late bool, invited []string) string {
	text := fmt.Sprintf("⚠️ %s não vai conseguir ir no %s: %s.", d.PersonName, d.MinistryName, slotText(d))
	if late {
		text += " Avisou com menos de 72h."
	}
	if len(invited) > 0 {
		return text + " Já chamei " + joinNames(invited) + " para substituir."
	}
	return text + " Ninguém da equipe está livre, vai precisar da sua ajuda 🙏"
}

func declinedManyHeader(d store.AssignmentDetail, why absence) string {
	header := fmt.Sprintf("⚠️ %s não vai conseguir ir nestes cultos do %s", d.PersonName, d.MinistryName)
	if why.period != "" {
		header = fmt.Sprintf("⚠️ %s não vai poder servir no %s %s", d.PersonName, d.MinistryName, why.period)
	}
	if why.reason != "" {
		header += " (" + why.reason + ")"
	}
	return header + ":"
}

func declinedManyLine(d store.AssignmentDetail, late bool, invited []string) string {
	line := "• " + slotText(d)
	if len(invited) > 0 {
		line += " — chamei " + joinNames(invited)
	} else {
		line += " — ninguém livre, vai precisar da sua ajuda"
	}
	if late {
		line += " ⏰ menos de 72h"
	}
	return line
}

func blockedNote(names []string) string {
	return "\n(Modo de teste: não chamei " + joinNames(names) + ".)"
}

func coveredReply(name string, slots []string) string {
	return fmt.Sprintf("Combinado, %s! 🙏 Você está na escala: %s.", firstName(name), joinNames(slots))
}

func replacedNotice(absent, substitute string, slots []string) string {
	return fmt.Sprintf("Boa notícia, %s! %s vai ficar no seu lugar: %s 🙌", firstName(absent), substitute, joinNames(slots))
}

func swapAnnouncement(ministry, substitute, absent string, slots []string) string {
	return fmt.Sprintf("🔄 Mudança na escala do %s: %s substitui %s — %s.", ministry, substitute, absent, joinNames(slots))
}

func alreadyTakenReply(name, substitute string, slots []string) string {
	return fmt.Sprintf("Obrigado pela disposição, %s! 🙏 %s já ficou com %s.", firstName(name), substitute, joinNames(slots))
}

func goneReply(all bool, slots []string) string {
	switch {
	case all:
		return "Obrigado pela disposição! 🙏 Esse já foi resolvido."
	case len(slots) == 1:
		return fmt.Sprintf("Obrigado! 🙏 %s já foi resolvido.", capitalize(slots[0]))
	}
	return fmt.Sprintf("Obrigado! 🙏 %s já foram resolvidos.", capitalize(joinNames(slots)))
}

func refusedReply() string {
	return "Tudo bem, obrigado por responder! 🙏"
}

func uncoveredAlert(d store.AssignmentDetail, absent string, slots []string) string {
	return fmt.Sprintf("❗ Ninguém conseguiu substituir %s no %s: %s. Vai precisar da sua ajuda nessa 🙏", absent, d.MinistryName, joinNames(slots))
}

func comebackTakenReply(d store.AssignmentDetail) string {
	return fmt.Sprintf("%s já ficou no seu lugar (%s). Se quiser voltar, fala com a liderança 🙂", d.PersonName, slotText(d))
}

func comebackReleased(name, returning string, d store.AssignmentDetail) string {
	return fmt.Sprintf("Não precisa mais, %s: %s vai conseguir ir (%s). Obrigado pela disposição! 🙏", firstName(name), returning, slotText(d))
}

func comebackAlert(returning string, d store.AssignmentDetail) string {
	return fmt.Sprintf("↩️ %s conseguiu e vai, afinal: %s, %s.", returning, slotText(d), d.MinistryName)
}
